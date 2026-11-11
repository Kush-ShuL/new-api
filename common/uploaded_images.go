package common

import (
	"regexp"
	"sync"
)

const UploadedImageDirectory = "uploads/backgrounds"
const UploadedImageURLPrefix = "/api/appearance/background/"

var UploadedImageName = regexp.MustCompile(`^[a-f0-9]{64}\.(png|jpg|gif|webp)$`)
var UploadedImageMutex sync.Mutex
