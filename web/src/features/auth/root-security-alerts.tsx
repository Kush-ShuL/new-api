import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { deleteUser } from '@/features/users/api'
import { api } from '@/lib/api'
import { handleServerError } from '@/lib/handle-server-error'
import { requireServerSuccess } from '@/lib/server-error-message'
import { useAuthStore } from '@/stores/auth-store'

import {
  SecureVerificationDialog,
  useSecureVerification,
} from './secure-verification'

interface BlockedRootAccount {
  id: number
  username: string
  status: number
}

export function RootSecurityAlerts() {
  const user = useAuthStore((state) => state.auth.user)
  if (user?.id !== 1 || user.role !== 100) return null
  return <RootSecurityReview />
}

function RootSecurityReview() {
  const { t } = useTranslation()
  const sessionID = useAuthStore((state) => state.auth.session?.sid)
  const verification = useSecureVerification()
  const [dismissed, setDismissed] = useState<number[]>([])
  const [deleting, setDeleting] = useState(false)
  const query = useQuery({
    queryKey: ['root-security', sessionID],
    queryFn: async () =>
      requireServerSuccess(
        (
          await api.get<{ success: boolean; data: BlockedRootAccount[] }>(
            '/api/user/self/root-security'
          )
        ).data
      ).data,
    refetchInterval: 60_000,
  })
  const account = query.data?.find((item) => !dismissed.includes(item.id))

  async function handleDelete() {
    if (!account) return
    setDeleting(true)
    try {
      const proof = await verification.requestVerification({
        scope: 'admin.user.delete',
        context: { user_id: account.id },
        title: t('Verify to delete user'),
        description: t(
          'Confirm your identity before permanently deleting the account {{username}}.',
          { username: account.username }
        ),
      })
      if (!proof) return
      requireServerSuccess(await deleteUser(account.id, proof.proof_token))
      toast.success(t('Deleted successfully'))
      await query.refetch()
    } catch (error) {
      handleServerError(error)
    } finally {
      setDeleting(false)
    }
  }

  return (
    <>
      <ConfirmDialog
        open={Boolean(account) && !verification.isActive}
        onOpenChange={(open) => {
          if (!open && account) setDismissed((ids) => [...ids, account.id])
        }}
        title={t('Unauthorized root account blocked')}
        desc={t(
          'Only user ID 1 can be root. Account {{username}} (ID {{id}}) has been blocked and its sessions revoked. Permanently delete this account?',
          { username: account?.username, id: account?.id }
        )}
        cancelBtnText={t('Keep blocked')}
        confirmText={t('Permanently delete')}
        destructive
        isLoading={deleting}
        handleConfirm={handleDelete}
      />
      <SecureVerificationDialog {...verification.dialogProps} />
    </>
  )
}
