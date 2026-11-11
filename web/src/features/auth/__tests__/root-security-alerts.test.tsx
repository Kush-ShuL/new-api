import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { RootSecurityAlerts } from '@/features/auth/root-security-alerts'
import { deleteUser } from '@/features/users/api'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

const verify = vi.hoisted(() => vi.fn())
vi.mock('@/features/auth/secure-verification', () => ({
  useSecureVerification: () => ({
    requestVerification: verify,
    isActive: false,
    dialogProps: {},
  }),
  SecureVerificationDialog: () => null,
}))
vi.mock('@/features/users/api', () => ({ deleteUser: vi.fn() }))

afterEach(() => {
  useAuthStore.setState(useAuthStore.getInitialState(), true)
  vi.restoreAllMocks()
  vi.clearAllMocks()
})

function showAlerts(role = 100, id = 1) {
  useAuthStore.getState().auth.setUser({ id, username: 'root', role })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <RootSecurityAlerts />
    </QueryClientProvider>
  )
}

test('does not query or expose alerts for ordinary users or a noncanonical root', () => {
  const get = vi.spyOn(api, 'get')
  const first = showAlerts(1)
  expect(get).not.toHaveBeenCalled()
  first.unmount()
  showAlerts(100, 2)
  expect(get).not.toHaveBeenCalled()
})

test('keeps a suspicious account blocked when the root dismisses the warning', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: [{ id: 2, username: 'rogue', status: 2 }],
    },
  })
  showAlerts()
  expect(await screen.findByRole('alertdialog')).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'Keep blocked' }))
  await waitFor(() =>
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  )
  expect(deleteUser).not.toHaveBeenCalled()
})

test('uses target-bound verification before deleting and does nothing if verification is canceled', async () => {
  const get = vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: [{ id: 2, username: 'rogue', status: 2 }] },
  })
  verify
    .mockResolvedValueOnce(null)
    .mockResolvedValueOnce({ proof_token: 'verified-proof' })
  vi.mocked(deleteUser).mockResolvedValue({ success: true })
  showAlerts()
  fireEvent.click(
    await screen.findByRole('button', { name: 'Permanently delete' })
  )
  await waitFor(() => expect(verify).toHaveBeenCalledTimes(1))
  expect(deleteUser).not.toHaveBeenCalled()
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Permanently delete' })
    ).toBeEnabled()
  )
  get.mockResolvedValue({ data: { success: true, data: [] } })
  fireEvent.click(screen.getByRole('button', { name: 'Permanently delete' }))
  await waitFor(() =>
    expect(deleteUser).toHaveBeenCalledWith(2, 'verified-proof')
  )
  expect(verify).toHaveBeenLastCalledWith(
    expect.objectContaining({
      scope: 'admin.user.delete',
      context: { user_id: 2 },
    })
  )
})
