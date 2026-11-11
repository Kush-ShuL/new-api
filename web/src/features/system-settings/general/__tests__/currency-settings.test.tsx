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
import { PricingSection } from '../pricing-section'

function Fixture() {
  const [container, setContainer] = useState<HTMLDivElement | null>(null)
  return (
    <>
      <div ref={setContainer} />
      <SettingsPageProvider actionsContainer={container}>
        <PricingSection
          defaultValues={{
            QuotaPerUnit: 500000,
            USDExchangeRate: 7,
            DisplayInCurrencyEnabled: true,
            DisplayTokenStatEnabled: true,
            general_setting: {
              quota_display_type: 'CUSTOM',
              custom_currency_symbol: 'Credits',
              custom_currency_exchange_rate: 1,
              custom_currency_icon: '',
            },
          }}
        />
      </SettingsPageProvider>
    </>
  )
}

test('uploading a currency icon waits for completion, saves the URL, and can clear it', async () => {
  const user = userEvent.setup()
  const url = `/api/appearance/background/${'b'.repeat(64)}.png`
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
  await screen.findByText('Custom Currency Icon')
  fireEvent.change(
    screen.getByLabelText('Upload file', { selector: 'input' }),
    {
      target: { files: [new File(['png'], 'coin.png', { type: 'image/png' })] },
    }
  )
  await waitFor(() => expect(post).toHaveBeenCalled())
  expect(screen.getByRole('button', { name: 'Save Changes' })).toBeDisabled()
  await act(async () =>
    finishUpload({ data: { success: true, data: { url } } })
  )
  expect(screen.getByRole('button', { name: 'Save Changes' })).toBeEnabled()
  expect(
    screen.getByRole('button', { name: 'Upload file' }).querySelector('img')
  ).toHaveAttribute('src', url)
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() =>
    expect(put).toHaveBeenCalledWith('/api/option/', {
      key: 'general_setting.custom_currency_icon',
      value: url,
    })
  )
  await user.click(screen.getByRole('button', { name: 'Clear' }))
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() =>
    expect(put).toHaveBeenCalledWith('/api/option/', {
      key: 'general_setting.custom_currency_icon',
      value: '',
    })
  )
})
