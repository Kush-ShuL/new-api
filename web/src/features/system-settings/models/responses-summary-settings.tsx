import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { Combobox } from '@/components/ui/combobox'
import { Label } from '@/components/ui/label'
import { getChannel } from '@/features/channels/api'
import { requireServerSuccess } from '@/lib/server-error-message'

import { getUpstreamChannels } from '../api'

type SummaryLevel = 'concise' | 'auto' | 'detailed'
type SummaryModel = { channel_id: number; model: string }
type SummaryModels = Record<SummaryLevel, SummaryModel>

function SummaryModelRow(props: {
  level: SummaryLevel
  label: string
  value: SummaryModel
  channels: { value: string; label: string }[]
  onChange: (value: SummaryModel) => void
}) {
  const { t } = useTranslation()
  const channelQuery = useQuery({
    queryKey: ['channel', props.value.channel_id],
    queryFn: async () =>
      requireServerSuccess(await getChannel(props.value.channel_id)),
    enabled: props.value.channel_id > 0,
  })
  const models = (channelQuery.data?.data?.models ?? '')
    .split(',')
    .filter(Boolean)
  return (
    <div className='grid min-w-0 gap-2 sm:grid-cols-[5rem_minmax(0,1fr)_minmax(0,1fr)] sm:items-center'>
      <span className='text-sm font-medium'>{props.label}</span>
      <div className='min-w-0'>
        <Label className='sr-only' htmlFor={`summary-${props.level}-channel`}>
          {t('Summary channel')} ({props.label})
        </Label>
        <Combobox
          id={`summary-${props.level}-channel`}
          options={props.channels}
          value={props.value.channel_id ? String(props.value.channel_id) : ''}
          onValueChange={(value) =>
            props.onChange({ channel_id: Number(value), model: '' })
          }
          placeholder={t('Summary channel')}
        />
      </div>
      <div className='min-w-0'>
        <Label className='sr-only' htmlFor={`summary-${props.level}-model`}>
          {t('Summary model')} ({props.label})
        </Label>
        <Combobox
          id={`summary-${props.level}-model`}
          options={models.map((model) => ({ value: model, label: model }))}
          value={props.value.model}
          onValueChange={(model) =>
            props.onChange({ ...props.value, model: model ?? '' })
          }
          placeholder={t('Summary model')}
          disabled={!props.value.channel_id || channelQuery.isPending}
        />
      </div>
    </div>
  )
}

export function ResponsesSummaryModels(props: {
  value: SummaryModels
  onChange: (value: SummaryModels) => void
}) {
  const { t } = useTranslation()
  const channelsQuery = useQuery({
    queryKey: ['upstream-channels'],
    queryFn: async () => requireServerSuccess(await getUpstreamChannels()),
  })
  const channels = (channelsQuery.data?.data ?? [])
    .filter((channel) => channel.id > 0 && channel.status === 1)
    .map((channel) => ({
      value: String(channel.id),
      label: `${channel.name} (#${channel.id})`,
    }))
  const labels = {
    concise: t('Concise'),
    auto: t('Auto'),
    detailed: t('Detailed'),
  }
  return (
    <div className='space-y-3'>
      {(['concise', 'auto', 'detailed'] as const).map((level) => (
        <SummaryModelRow
          key={level}
          level={level}
          label={labels[level]}
          value={props.value[level]}
          channels={channels}
          onChange={(value) =>
            props.onChange({ ...props.value, [level]: value })
          }
        />
      ))}
    </div>
  )
}
