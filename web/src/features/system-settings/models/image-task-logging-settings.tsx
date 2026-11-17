import { useTranslation } from 'react-i18next'

import { SettingsSwitchField } from '../components/settings-form-layout'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import type { ModelSettings } from '../types'

const fields = [
  { key: 'global.image_log_prompt', label: 'Log image prompts' },
  { key: 'global.image_log_input', label: 'Log input images' },
  { key: 'global.image_log_output', label: 'Log output images' },
] as const

export function ImageTaskLoggingSettings(props: { settings: ModelSettings }) {
  const { t } = useTranslation()
  const update = useUpdateOption()
  return (
    <SettingsSection title={t('Image Task Logging')}>
      {fields.map(({ key, label }) => (
        <SettingsSwitchField
          key={key}
          controlId={key}
          checked={props.settings[key] === true}
          label={t(label)}
          disabled={update.isPending}
          onCheckedChange={(checked) => update.mutate({ key, value: checked })}
        />
      ))}
    </SettingsSection>
  )
}
