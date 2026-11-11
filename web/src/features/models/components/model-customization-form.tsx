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
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

import { ImageUpload } from '@/components/image-upload'
import { ModelIdentity } from '@/components/model-identity'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useUpdateOption } from '@/features/system-settings/hooks/use-update-option'
import {
  useModelCustomizations,
  type ModelCustomization,
} from '@/hooks/use-model-customizations'
import { getLobeIcon } from '@/lib/lobe-icon'
import { uploadBackground } from '@/lib/upload-background'

const DEFAULT: ModelCustomization = {
  display_name: '',
  icon: '',
  background: '',
}

const schema = z.object({
  display_name: z.string().max(128),
  icon: z.string(),
  background: z.string().max(4096),
})

export function ModelCustomizationForm(props: {
  modelName: string
  onDirtyChange: (dirty: boolean) => void
}) {
  const { t } = useTranslation()
  const saved = useModelCustomizations()
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: { ...DEFAULT, ...saved[props.modelName] },
  })
  const draft = form.watch()
  const name = props.modelName
  const update = useUpdateOption()
  const dirty = form.formState.isDirty
  const onDirtyChange = props.onDirtyChange
  useEffect(() => onDirtyChange(dirty), [dirty, onDirtyChange])

  return (
    <div className='min-h-0 flex-1 overflow-y-auto p-4'>
      <form
        className='mx-auto max-w-2xl space-y-6'
        onSubmit={form.handleSubmit((values) => {
          update.mutate(
            {
              key: 'ModelCustomizations',
              value: JSON.stringify({
                ...saved,
                [name]: {
                  display_name: values.display_name,
                  icon: values.icon,
                  background: values.background,
                },
              }),
            },
            { onSuccess: () => form.reset(values) }
          )
        })}
      >
        <ModelIdentity name={name} showId />
        <div className='space-y-2'>
          <Label htmlFor='custom-display-name'>{t('Display name')}</Label>
          <Input
            id='custom-display-name'
            maxLength={128}
            {...form.register('display_name')}
          />
        </div>
        <ImageUpload
          value={draft.icon}
          preview={getLobeIcon(draft.icon, 28)}
          onChange={(icon) =>
            form.setValue('icon', icon, { shouldDirty: true })
          }
          label={t('Upload model icon')}
          maxBytes={128 * 1024}
        />
        <div className='space-y-3'>
          <Label htmlFor='model-card-background'>
            {t('Model card background')}
          </Label>
          <Input
            id='model-card-background'
            placeholder='https://example.com/model.webp'
            {...form.register('background')}
          />
          <ImageUpload
            value={draft.background}
            label={t('Upload background image')}
            maxBytes={100 * 1024 * 1024}
            onUpload={uploadBackground}
            onChange={(background) =>
              form.setValue('background', background, { shouldDirty: true })
            }
          />
          <p className='text-muted-foreground text-sm'>
            {t(
              'Used behind the model card and as a large image in model details.'
            )}
          </p>
        </div>
        <div className='flex gap-2'>
          <Button type='submit' disabled={!name.trim() || update.isPending}>
            {t('Save Changes')}
          </Button>
          {saved[name] && (
            <Button
              variant='outline'
              type='button'
              disabled={update.isPending}
              onClick={() => {
                const next = { ...saved }
                delete next[name]
                update.mutate(
                  { key: 'ModelCustomizations', value: JSON.stringify(next) },
                  { onSuccess: () => form.reset(DEFAULT) }
                )
              }}
            >
              {t('Reset')}
            </Button>
          )}
        </div>
      </form>
    </div>
  )
}
