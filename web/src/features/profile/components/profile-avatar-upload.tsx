import { Camera, LoaderCircle } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ImageUpload } from '@/components/image-upload'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { api } from '@/lib/api'
import { getUserAvatarFallback, getUserAvatarStyle } from '@/lib/avatar'
import { requireServerSuccess } from '@/lib/server-error-message'
import { uploadBackground } from '@/lib/upload-background'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import type { UserProfile } from '../types'

export function ProfileAvatarUpload(props: {
  profile: UserProfile
  onUpdate: (profile: UserProfile) => void
}) {
  const { t } = useTranslation()
  const name = props.profile.username || props.profile.display_name || ''

  async function save(url: string) {
    const response = requireServerSuccess(
      (
        await api.put<{ success: boolean; data: UserProfile }>(
          '/api/user/self/avatar',
          { url }
        )
      ).data
    )
    useAuthStore.getState().auth.setUser(response.data)
    props.onUpdate(response.data)
    toast.success(t('Profile updated successfully'))
  }

  return (
    <ImageUpload
      value={props.profile.avatar_url ?? ''}
      onChange={save}
      label={t('Upload avatar')}
      maxBytes={5 * 1024 * 1024}
      clearable={false}
      onUpload={(file) => uploadBackground(file, '/api/user/self/avatar/image')}
      triggerClassName='group relative h-auto rounded-xl border-0 p-0 hover:bg-transparent sm:rounded-2xl'
      renderTrigger={(uploading) => (
        <>
          <Avatar className='ring-background h-12 w-12 rounded-xl text-sm ring-2 sm:h-16 sm:w-16 sm:rounded-2xl sm:text-lg sm:ring-4'>
            <AvatarImage
              src={props.profile.avatar_url}
              alt={props.profile.display_name || name}
              className='rounded-[inherit]'
            />
            <AvatarFallback
              className='rounded-xl font-semibold text-white sm:rounded-2xl'
              style={getUserAvatarStyle(name)}
            >
              {getUserAvatarFallback(name)}
            </AvatarFallback>
          </Avatar>
          <span
            aria-hidden='true'
            className={cn(
              'absolute inset-0 flex items-center justify-center rounded-[inherit] bg-black/40 text-white opacity-0 transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100',
              uploading && 'opacity-100'
            )}
          >
            {uploading ? <LoaderCircle className='animate-spin' /> : <Camera />}
          </span>
        </>
      )}
    />
  )
}
