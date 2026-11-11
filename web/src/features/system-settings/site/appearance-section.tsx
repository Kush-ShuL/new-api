/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { zodResolver } from '@hookform/resolvers/zod'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { AppearanceSettings } from '@/components/appearance-settings'
import { Button } from '@/components/ui/button'
import { Form } from '@/components/ui/form'
import {
  appearanceSchema,
  DEFAULT_APPEARANCE,
  type Appearance,
} from '@/lib/appearance'

import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import { SettingsForm } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

export function AppearanceSection(props: { defaultValue: string }) {
  const { t } = useTranslation()
  const update = useUpdateOption()
  const [defaults] = useState<Appearance>(() => {
    if (!props.defaultValue) return DEFAULT_APPEARANCE
    try {
      const result = appearanceSchema.safeParse(JSON.parse(props.defaultValue))
      return result.success ? result.data : DEFAULT_APPEARANCE
    } catch {
      return DEFAULT_APPEARANCE
    }
  })
  const [browserAppearance] = useState(() => {
    try {
      const legacy = JSON.parse(
        localStorage.getItem('newapi-appearance') || 'null'
      )
      const result = appearanceSchema.safeParse({
        ...DEFAULT_APPEARANCE,
        ...legacy?.state,
      })
      return result.success && result.data.background ? result.data : null
    } catch {
      return null
    }
  })
  const { form, handleSubmit, handleReset, isDirty, isSubmitting } =
    useSettingsForm({
      resolver: zodResolver(appearanceSchema),
      defaultValues: defaults,
      onSubmit: async (values) => {
        await update.mutateAsync({
          key: 'AppearanceSettings',
          value: JSON.stringify(values),
        })
      },
    })
  const draft = form.watch()
  function edit(next: Appearance): void {
    for (const key of Object.keys(next) as (keyof Appearance)[]) {
      form.setValue(key, next[key], { shouldDirty: true })
    }
  }

  return (
    <SettingsSection title={t('Appearance & Background')} className='max-w-2xl'>
      <FormNavigationGuard when={isDirty} />
      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsPageFormActions
            onSave={handleSubmit}
            onReset={handleReset}
            isSaving={isSubmitting || update.isPending}
            isResetDisabled={!isDirty}
          />
          <FormDirtyIndicator isDirty={isDirty} />
          <p className='text-muted-foreground text-sm'>
            {t('Saved appearance settings apply to all pages and visitors.')}
          </p>
          {!props.defaultValue && browserAppearance && (
            <Button
              type='button'
              variant='outline'
              className='w-fit'
              onClick={() => edit(browserAppearance)}
            >
              {t('Import browser appearance')}
            </Button>
          )}
          <AppearanceSettings value={draft} onChange={edit} />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
