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
import { useTranslation } from 'react-i18next'

import { ImageUpload } from '@/components/image-upload'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Slider } from '@/components/ui/slider'
import { Switch } from '@/components/ui/switch'
import type { Appearance } from '@/lib/appearance'
import { uploadBackground } from '@/lib/upload-background'

export function AppearanceSettings(props: {
  value: Appearance
  onChange: (value: Appearance) => void
}) {
  const { t } = useTranslation()
  const appearance = props.value
  let backgroundURL = appearance.background
  if (backgroundURL.startsWith('data:')) backgroundURL = ''
  if (backgroundURL.startsWith('/')) {
    backgroundURL = new URL(backgroundURL, window.location.origin).href
  }
  const update = (value: Partial<Appearance>) =>
    props.onChange({ ...appearance, ...value })
  return (
    <section className='space-y-5'>
      <div className='flex items-center justify-between gap-3'>
        <Label htmlFor='experimental-glass'>
          {t('Frosted glass UI (experimental)')}
        </Label>
        <Switch
          id='experimental-glass'
          checked={appearance.glassEnabled}
          onCheckedChange={(glassEnabled) => update({ glassEnabled })}
        />
      </div>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Custom backgrounds are available when frosted glass UI is enabled.'
        )}
      </p>
      <div className='space-y-2'>
        <Label htmlFor='background-url'>{t('Background image URL')}</Label>
        <Input
          id='background-url'
          type='url'
          placeholder='https://example.com/background.webp'
          value={backgroundURL}
          disabled={!appearance.glassEnabled}
          onChange={(event) => update({ background: event.target.value })}
        />
      </div>
      <ImageUpload
        label={t('Upload background image')}
        value={appearance.background}
        maxBytes={100 * 1024 * 1024}
        disabled={!appearance.glassEnabled}
        onChange={(background) => update({ background })}
        onUpload={uploadBackground}
      />
      <p className='text-muted-foreground text-sm'>
        {t('Upload an image up to 100 MB, or use an image URL.')}
      </p>
      <div className='space-y-4'>
        <div className='space-y-3'>
          <Label htmlFor='background-opacity'>
            {t('Image opacity')}{' '}
            <span className='ml-auto tabular-nums'>
              {appearance.imageOpacity}%
            </span>
          </Label>
          <Slider
            id='background-opacity'
            aria-label={t('Image opacity')}
            min={0}
            max={100}
            value={[appearance.imageOpacity]}
            disabled={!appearance.glassEnabled}
            onValueChange={(value) =>
              update({
                imageOpacity: Number(Array.isArray(value) ? value[0] : value),
              })
            }
          />
        </div>
        <div className='space-y-3'>
          <Label htmlFor='background-mask'>
            {t('Mask strength')}{' '}
            <span className='ml-auto tabular-nums'>
              {appearance.maskOpacity}%
            </span>
          </Label>
          <Slider
            id='background-mask'
            aria-label={t('Mask strength')}
            min={0}
            max={100}
            value={[appearance.maskOpacity]}
            disabled={!appearance.glassEnabled}
            onValueChange={(value) =>
              update({
                maskOpacity: Number(Array.isArray(value) ? value[0] : value),
              })
            }
          />
        </div>
        <div className='space-y-3'>
          <Label htmlFor='surface-opacity'>
            {t('Panel opacity')}{' '}
            <span className='ml-auto tabular-nums'>
              {appearance.surfaceOpacity}%
            </span>
          </Label>
          <Slider
            id='surface-opacity'
            aria-label={t('Panel opacity')}
            min={0}
            max={100}
            value={[appearance.surfaceOpacity]}
            disabled={!appearance.glassEnabled}
            onValueChange={(value) =>
              update({
                surfaceOpacity: Number(Array.isArray(value) ? value[0] : value),
              })
            }
          />
        </div>
        <div className='space-y-3'>
          <Label htmlFor='glass-blur'>
            {t('Blur strength')}
            <span className='ml-auto tabular-nums'>
              {appearance.blurStrength} px
            </span>
          </Label>
          <Slider
            id='glass-blur'
            aria-label={t('Blur strength')}
            min={0}
            max={60}
            value={[appearance.blurStrength]}
            disabled={!appearance.glassEnabled}
            onValueChange={(value) =>
              update({
                blurStrength: Number(Array.isArray(value) ? value[0] : value),
              })
            }
          />
        </div>
      </div>
    </section>
  )
}
