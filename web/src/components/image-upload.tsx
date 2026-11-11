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
import {
  useId,
  useRef,
  useState,
  type ChangeEvent,
  type ReactNode,
} from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { handleServerError } from '@/lib/handle-server-error'

export function ImageUpload(props: {
  value: string
  onChange: (value: string) => void | Promise<void>
  label: string
  maxBytes: number
  disabled?: boolean
  onUpload?: (file: File) => Promise<string>
  onUploadingChange?: (uploading: boolean) => void
  preview?: ReactNode
  renderTrigger?: (uploading: boolean) => ReactNode
  triggerClassName?: string
  clearable?: boolean
}) {
  const { t } = useTranslation()
  const input = useRef<HTMLInputElement>(null)
  const id = useId()
  const [uploading, setUploading] = useState(false)

  async function upload(event: ChangeEvent<HTMLInputElement>): Promise<void> {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return
    if (
      !['image/png', 'image/jpeg', 'image/webp', 'image/gif'].includes(
        file.type
      ) ||
      file.size > props.maxBytes
    ) {
      const message =
        props.maxBytes >= 1024 * 1024
          ? t('Choose a PNG, JPEG, WebP, or GIF image up to {{size}} MB.', {
              size: props.maxBytes / (1024 * 1024),
            })
          : t(
              'Choose a PNG, JPEG, WebP, or GIF image smaller than {{size}} KB.',
              { size: Math.round(props.maxBytes / 1024) }
            )
      toast.error(message)
      return
    }
    if (props.onUpload) {
      setUploading(true)
      props.onUploadingChange?.(true)
      try {
        await props.onChange(await props.onUpload(file))
      } catch (error) {
        handleServerError(error, t('Failed to upload image'))
      } finally {
        setUploading(false)
        props.onUploadingChange?.(false)
      }
      return
    }
    const reader = new FileReader()
    reader.addEventListener(
      'load',
      () => props.onChange(String(reader.result)),
      { once: true }
    )
    reader.addEventListener(
      'error',
      () => toast.error(t('Failed to read image')),
      { once: true }
    )
    reader.readAsDataURL(file)
  }

  return (
    <div className='flex flex-wrap items-center gap-3'>
      <Input
        ref={input}
        id={id}
        type='file'
        accept='image/png,image/jpeg,image/webp,image/gif'
        className='hidden'
        disabled={props.disabled || uploading}
        onChange={(event) => void upload(event)}
        aria-label={props.label}
      />
      <Button
        type='button'
        variant='outline'
        className={props.triggerClassName}
        disabled={props.disabled || uploading}
        aria-label={props.renderTrigger ? props.label : undefined}
        aria-busy={uploading || undefined}
        aria-controls={id}
        onClick={() => input.current?.click()}
      >
        {props.renderTrigger?.(uploading)}
        {!props.renderTrigger &&
          props.value &&
          (props.preview ?? (
            <img
              src={props.value}
              alt=''
              className='size-7 rounded object-cover'
            />
          ))}
        {!props.renderTrigger && (uploading ? t('Uploading...') : props.label)}
      </Button>
      {props.value && props.clearable !== false && (
        <Button
          type='button'
          variant='ghost'
          disabled={props.disabled || uploading}
          onClick={() => props.onChange('')}
        >
          {t('Clear')}
        </Button>
      )}
    </div>
  )
}
