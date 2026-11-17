package service

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/MoeForge/moego/ojson"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

const imageTaskCaptureKey = "image_task_capture"
const ImageRelayTaskPlatform = "image-relay"
const maxImageTaskLogBytesPerDirection = 12 << 20
const maxLoggedImagePrompt = 1024 << 10

type ImageTaskContent struct {
	Prompt       string   `json:"prompt,omitempty"`
	InputImages  []string `json:"input_images,omitempty"`
	OutputImages []string `json:"output_images,omitempty"`
}

type imageTaskCapture struct {
	body            ImageTaskContent
	outputEnabled   bool
	outputRemaining int
	inputTokens     int
	outputTokens    int
	quota           int
	durable         bool
}

func imageTaskLogOption(key string) bool {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	return common.OptionMap[key] == "true"
}

func BeginImageTaskLog(c *gin.Context, request *dto.ImageRequest) {
	capture := &imageTaskCapture{outputEnabled: imageTaskLogOption("global.image_log_output"), outputRemaining: maxImageTaskLogBytesPerDirection}
	c.Set(imageTaskCaptureKey, capture)
	if imageTaskLogOption("global.image_log_prompt") {
		capture.body.Prompt = strings.ToValidUTF8(request.Prompt[:min(len(request.Prompt), maxLoggedImagePrompt)], "�")
	}
	if !imageTaskLogOption("global.image_log_input") {
		return
	}
	remaining := maxImageTaskLogBytesPerDirection
	appendImage := func(item ojson.Result) {
		var value string
		if item.Type == ojson.String {
			value = item.String()
		} else {
			for _, path := range []string{"image_url", "url", "image_url.url", "b64_json"} {
				if field := item.Get(path); field.Type == ojson.String {
					value = field.String()
					break
				}
			}
		}
		addLoggedImage(&capture.body.InputImages, &remaining, value)
	}
	appendImages := func(result ojson.Result) {
		if result.IsArray() {
			result.ForEach(func(_, item ojson.Result) bool { appendImage(item); return remaining > 0 })
		} else {
			appendImage(result)
		}
	}
	if strings.HasPrefix(c.ContentType(), "multipart/form-data") {
		form := c.Request.MultipartForm
		if form == nil {
			var err error
			form, err = common.ParseMultipartFormReusable(c)
			if err != nil {
				return
			}
			defer form.RemoveAll()
		}
		for _, key := range []string{"image", "image[]", "images", "images[]", "mask"} {
			for _, value := range form.Value[key] {
				if strings.HasPrefix(value, "[") || strings.HasPrefix(value, "{") || strings.HasPrefix(value, `"`) {
					appendImages(ojson.Parse(value))
				} else {
					addLoggedImage(&capture.body.InputImages, &remaining, value)
				}
			}
			for _, file := range form.File[key] {
				if file.Size > int64(remaining/4*3) || len(capture.body.InputImages) >= 32 {
					continue
				}
				part, err := file.Open()
				if err != nil {
					continue
				}
				data, err := io.ReadAll(io.LimitReader(part, file.Size+1))
				_ = part.Close()
				if err == nil && int64(len(data)) == file.Size {
					addLoggedImage(&capture.body.InputImages, &remaining, base64.StdEncoding.EncodeToString(data))
				}
			}
		}
		return
	}
	for _, raw := range []common.RawMessage{request.Image, request.Images, request.Mask} {
		appendImages(ojson.ParseBytes(raw))
	}
}

// BeginPluginImageTaskLog uses the already decoded plugin request; uploads are
// read from the reusable form without changing the plugin payload.
func BeginPluginImageTaskLog(c *gin.Context, body any, raw common.RawMessage) {
	var request dto.ImageRequest
	if imageTaskLogOption("global.image_log_prompt") || imageTaskLogOption("global.image_log_input") {
		if len(raw) > 0 {
			_ = common.Unmarshal(raw, &request)
		} else if fields, ok := body.(map[string]any); ok {
			request.Prompt, _ = fields["prompt"].(string)
		}
	}
	BeginImageTaskLog(c, &request)
}

func addLoggedImage(target *[]string, remaining *int, value string) {
	if value == "" || len(value) > *remaining || len(*target) >= 32 {
		return
	}
	if strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://") {
		u, err := url.Parse(value)
		if err != nil || u.Hostname() == "" || u.User != nil || len(value) > 8192 {
			return
		}
	} else {
		data := value
		if strings.HasPrefix(data, "data:") {
			_, encoded, ok := strings.Cut(data, ";base64,")
			if !ok {
				return
			}
			data = encoded
		}
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return
		}
		mime := http.DetectContentType(decoded)
		switch mime {
		case "image/png", "image/jpeg", "image/webp", "image/gif":
			value = "data:" + mime + ";base64," + data
		default:
			return
		}
	}
	if len(value) <= *remaining {
		*remaining -= len(value)
		*target = append(*target, value)
	}
}

// CaptureImageTaskOutput only reads completed image payloads and does no work
// when the operator has disabled output logging.
func CaptureImageTaskOutput(c *gin.Context, data []byte) {
	log, ok := common.GetContextKeyType[*imageTaskCapture](c, imageTaskCaptureKey)
	if !ok || !log.outputEnabled || log.outputRemaining == 0 {
		return
	}
	root := ojson.ParseBytes(data)
	appendImage := func(item ojson.Result) {
		for _, field := range []string{"b64_json", "url"} {
			if value := item.Get(field); value.Type == ojson.String && len(value.Raw) <= log.outputRemaining+2 {
				before := len(log.body.OutputImages)
				addLoggedImage(&log.body.OutputImages, &log.outputRemaining, value.String())
				if len(log.body.OutputImages) != before {
					return
				}
			}
		}
	}
	switch root.Get("type").String() {
	case "image_generation.completed", "image_edit.completed":
		appendImage(root)
	default:
		items := root.Get("data")
		if items.IsObject() {
			appendImage(items)
		} else if items.IsArray() {
			items.ForEach(func(_, item ojson.Result) bool { appendImage(item); return log.outputRemaining > 0 })
		}
	}
}

func captureImageTaskUsage(c *gin.Context, input, output, quota int) {
	if log, ok := common.GetContextKeyType[*imageTaskCapture](c, imageTaskCaptureKey); ok {
		log.inputTokens, log.outputTokens, log.quota = input, output, quota
	}
}

func AddImageTaskLog(c *gin.Context, task *model.Task, start time.Time) {
	log, ok := common.GetContextKeyType[*imageTaskCapture](c, imageTaskCaptureKey)
	if !ok {
		return
	}
	task.PrivateData.ImageLog = &model.ImageTaskLogMeta{
		Endpoint: c.Request.URL.Path, IP: c.ClientIP(), DurationMS: time.Since(start).Milliseconds(),
		InputTokens: log.inputTokens, OutputTokens: log.outputTokens,
		HasContent: log.body.Prompt != "" || len(log.body.InputImages)+len(log.body.OutputImages) > 0,
	}
	if task.PrivateData.ImageLog.HasContent {
		task.ImageLogContent, _ = common.Marshal(log.body)
	}
}

func MarkImageTaskLogDurable(c *gin.Context) {
	if log, ok := common.GetContextKeyType[*imageTaskCapture](c, imageTaskCaptureKey); ok {
		log.durable = true
	}
}

func CompletePluginImageTaskLog(c *gin.Context, task *model.Task, start time.Time, response map[string]any) {
	log, ok := common.GetContextKeyType[*imageTaskCapture](c, imageTaskCaptureKey)
	if !ok || !log.durable || task.PrivateData.ImageLog == nil {
		return
	}
	if log.outputEnabled {
		if data, err := common.Marshal(response["data"]); err == nil {
			CaptureImageTaskOutput(c, []byte(`{"data":`+string(data)+`}`))
		}
	}
	if data, err := common.Marshal(response["usage"]); err == nil {
		usage := ojson.ParseBytes(data)
		log.inputTokens = int(max(0, usage.Get("input_tokens").Int()))
		log.outputTokens = int(max(0, usage.Get("output_tokens").Int()))
	}
	AddImageTaskLog(c, task, start)
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 5*time.Second)
	defer cancel()
	if err := model.DB.WithContext(ctx).Model(&model.Task{}).Where("id = ?", task.ID).Updates(map[string]any{
		"private_data": task.PrivateData, "image_log_content": task.ImageLogContent,
	}).Error; err != nil {
		logger.LogError(c, common.LogText("failed to update image task log"))
	}
}

func RecordImageTaskLog(c *gin.Context, info *relaycommon.RelayInfo, apiErr *types.NewAPIError) {
	log, ok := common.GetContextKeyType[*imageTaskCapture](c, imageTaskCaptureKey)
	if !ok || log.durable {
		return
	}
	finished := time.Now()
	path := c.Request.URL.Path
	if apiErr != nil && log.inputTokens == 0 {
		log.inputTokens = info.GetEstimatePromptTokens()
	}
	task := &model.Task{
		TaskID: model.GenerateTaskID(), Platform: ImageRelayTaskPlatform, UserId: info.UserId, Group: info.UsingGroup,
		Action: "image_generate", Status: model.TaskStatusSuccess, Progress: "100%",
		SubmitTime: info.StartTime.Unix(), StartTime: info.StartTime.Unix(), FinishTime: finished.Unix(), Quota: log.quota,
		Properties:  model.Properties{OriginModelName: info.OriginModelName},
		PrivateData: model.TaskPrivateData{Execution: &model.TaskExecutionSnapshot{RequestID: c.GetString(common.RequestIdKey), RequestPath: path}},
	}
	if info.ChannelMeta != nil {
		task.ChannelId = info.ChannelId
		task.Properties.UpstreamModelName = info.UpstreamModelName
	}
	if path == "/v1/images/edits" {
		task.Action = "image_edit"
	}
	if apiErr != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = string(apiErr.GetErrorCode())
	} else if status := info.StreamStatus; status != nil && (!status.IsNormalEnd() || status.HasErrors()) {
		task.Status = model.TaskStatusFailure
		task.FailReason = string(status.EndReason)
	}
	AddImageTaskLog(c, task, info.StartTime)
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 5*time.Second)
	defer cancel()
	if err := task.InsertWithContext(ctx); err != nil {
		// Do not log the failed insert or its payload, which may contain images.
		logger.LogError(c, common.LogText("failed to write image task log"))
	}
}
