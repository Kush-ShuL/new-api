import { z } from 'zod'

const summaryModelSchema = z.object({
  channel_id: z.number().default(0),
  model: z.string().default(''),
})

export const responsesSummaryModelsSchema = z.object({
  concise: summaryModelSchema.default({ channel_id: 0, model: '' }),
  auto: summaryModelSchema.default({ channel_id: 0, model: '' }),
  detailed: summaryModelSchema.default({ channel_id: 0, model: '' }),
})
