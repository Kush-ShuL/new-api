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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { SystemInfoSection } from '../system-info-section'

function Fixture() {
  const [container, setContainer] = useState<HTMLDivElement | null>(null)
  return (
    <>
      <div ref={setContainer} />
      <SettingsPageProvider actionsContainer={container}>
        <SystemInfoSection
          defaultValues={{
            SystemName: 'New API',
            ServerAddress: '',
            TaskPublicAddress: '',
            Logo: '',
            Footer: '',
            About: '',
            HomePageContent: '',
            general_setting: { docs_link: '' },
            legal: { user_agreement: '', privacy_policy: '' },
          }}
        />
      </SettingsPageProvider>
    </>
  )
}

test('uploads a logo, saves its local URL, and still accepts external URLs', async () => {
  const user = userEvent.setup()
  const uploadedUrl = `/api/appearance/background/${'a'.repeat(64)}.png`
  let finishUpload!: (value: unknown) => void
  const post = vi.spyOn(api, 'post').mockReturnValue(
    new Promise((resolve) => {
      finishUpload = resolve
    }) as ReturnType<typeof api.post>
  )
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  const router = createRouter({
    routeTree: createRootRoute({ component: Fixture }),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  const logo = await screen.findByRole('textbox', { name: 'Logo' })
  const file = new File(['image'], 'site.png', { type: 'image/png' })
  fireEvent.change(
    screen.getByLabelText('Upload file', { selector: 'input' }),
    { target: { files: [file] } }
  )
  await waitFor(() => expect(post).toHaveBeenCalled())
  expect(screen.getByRole('button', { name: 'Save Changes' })).toBeDisabled()
  expect(post.mock.calls[0][0]).toBe('/api/option/appearance/background')
  expect(post.mock.calls[0][1]).toBeInstanceOf(FormData)
  await act(async () =>
    finishUpload({ data: { success: true, data: { url: uploadedUrl } } })
  )
  expect(logo).toHaveValue(uploadedUrl)
  expect(screen.getByRole('button', { name: 'Save Changes' })).toBeEnabled()
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() =>
    expect(put).toHaveBeenCalledWith('/api/option/', {
      key: 'Logo',
      value: uploadedUrl,
    })
  )
  await user.clear(logo)
  await user.type(logo, 'https://example.com/logo.png')
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() =>
    expect(put).toHaveBeenCalledWith('/api/option/', {
      key: 'Logo',
      value: 'https://example.com/logo.png',
    })
  )
})
