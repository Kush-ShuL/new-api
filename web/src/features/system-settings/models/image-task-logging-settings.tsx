import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SettingsSwitchField } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import type { ModelSettings } from '../types'

const fields = [
  { key: 'global.image_log_prompt', label: 'Log image prompts' },
  { key: 'global.image_log_input', label: 'Log input images' },
  { key: 'global.image_log_output', label: 'Log output images' },
] as const

type ImageLogKey = (typeof fields)[number]['key']

export function ImageTaskLoggingSettings(props: { settings: ModelSettings }) {
  const { t } = useTranslation()
  const update = useUpdateOption()
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<Partial<Record<ImageLogKey, boolean>>>({})
  const [saving, setSaving] = useState(false)
  const changes = fields.flatMap(({ key }) => {
    const value = draft[key]
    return value !== undefined && value !== (props.settings[key] === true)
      ? [{ key, value }]
      : []
  })

  const save = async () => {
    setSaving(true)
    try {
      for (const change of changes) await update.mutateAsync(change)
      await queryClient.invalidateQueries({ queryKey: ['system-options'] })
      setDraft({})
    } catch {
      // The option mutation reports errors. Keep any unsaved edits visible.
    } finally {
      setSaving(false)
    }
  }

  return (
    <SettingsSection title={t('Image Task Logging')}>
      <SettingsPageFormActions
        onSave={save}
        isSaving={saving}
        isSaveDisabled={changes.length === 0}
      />
      {fields.map(({ key, label }) => (
        <SettingsSwitchField
          key={key}
          controlId={key}
          checked={draft[key] ?? props.settings[key] === true}
          label={t(label)}
          disabled={saving}
          onCheckedChange={(checked) =>
            setDraft((current) => ({ ...current, [key]: checked }))
          }
        />
      ))}
    </SettingsSection>
  )
}
