import { UserRound } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ImageUpload } from '@/components/image-upload'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import { TitledCard } from '@/components/ui/titled-card'
import { api } from '@/lib/api'
import { getUserAvatarFallback, getUserAvatarStyle } from '@/lib/avatar'
import { handleServerError } from '@/lib/handle-server-error'
import { requireServerSuccess } from '@/lib/server-error-message'
import { uploadBackground } from '@/lib/upload-background'
import { useAuthStore } from '@/stores/auth-store'

import type { UserProfile } from '../types'

export function ProfileAvatarCard(props: {
  profile: UserProfile | null
  onUpdate: () => void
}) {
  if (!props.profile) return null
  return (
    <AvatarEditor
      key={props.profile.avatar_url ?? ''}
      profile={props.profile}
      onUpdate={props.onUpdate}
    />
  )
}

function AvatarEditor(props: { profile: UserProfile; onUpdate: () => void }) {
  const { t } = useTranslation()
  const [url, setURL] = useState(props.profile.avatar_url ?? '')
  const [uploading, setUploading] = useState(false)
  const [saving, setSaving] = useState(false)
  const name = props.profile.display_name || props.profile.username

  async function save() {
    setSaving(true)
    try {
      const response = requireServerSuccess(
        (
          await api.put<{ success: boolean; data: UserProfile }>(
            '/api/user/self/avatar',
            { url }
          )
        ).data
      )
      useAuthStore.getState().auth.setUser(response.data)
      props.onUpdate()
      toast.success(t('Profile updated successfully'))
    } catch (error) {
      handleServerError(error)
    } finally {
      setSaving(false)
    }
  }

  return (
    <TitledCard
      title={t('Profile photo')}
      description={t(
        'Choose a PNG, JPEG, WebP, or GIF image up to {{size}} MB.',
        { size: 5 }
      )}
      icon={<UserRound />}
      disableHoverEffect
    >
      <div className='flex flex-wrap items-center gap-4'>
        <Avatar className='size-16'>
          <AvatarImage src={url} alt={name} />
          <AvatarFallback style={getUserAvatarStyle(name)}>
            {getUserAvatarFallback(name)}
          </AvatarFallback>
        </Avatar>
        <ImageUpload
          value={url}
          onChange={setURL}
          label={t('Upload avatar')}
          maxBytes={5 * 1024 * 1024}
          disabled={saving}
          onUploadingChange={setUploading}
          onUpload={(file) =>
            uploadBackground(file, '/api/user/self/avatar/image')
          }
        />
        <Button
          disabled={
            uploading || saving || url === (props.profile.avatar_url ?? '')
          }
          onClick={() => void save()}
        >
          {saving ? t('Saving...') : t('Save')}
        </Button>
      </div>
    </TitledCard>
  )
}
