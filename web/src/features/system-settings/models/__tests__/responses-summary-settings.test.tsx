import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { useState } from 'react'
import { I18nextProvider } from 'react-i18next'
import { afterEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { ResponsesSummaryModels } from '../responses-summary-settings'

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  initAsync: false,
})

afterEach(cleanup)

it('keeps summary levels independent and clears only the changed channel model', async () => {
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data:
        url === '/api/ratio_sync/channels'
          ? [
              { id: 101, name: 'Primary', status: 1 },
              { id: 102, name: 'Secondary', status: 1 },
            ]
          : {
              models:
                url === '/api/channel/101'
                  ? 'fast,balanced'
                  : 'detailed,alternate',
            },
    },
  }))
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const onChange = vi.fn()
  function Fixture() {
    const [value, setValue] = useState({
      concise: { channel_id: 101, model: 'fast' },
      auto: { channel_id: 101, model: 'balanced' },
      detailed: { channel_id: 102, model: 'detailed' },
    })
    return (
      <ResponsesSummaryModels
        value={value}
        onChange={(next) => {
          setValue(next)
          onChange(next)
        }}
      />
    )
  }
  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <Fixture />
      </QueryClientProvider>
    </I18nextProvider>
  )
  const user = userEvent.setup()
  const detailed = screen.getByRole('combobox', {
    name: 'Summary model (Detailed)',
  })
  await waitFor(() => expect(detailed).toBeEnabled())
  await user.click(detailed)
  await user.click(await screen.findByRole('option', { name: 'alternate' }))
  expect(onChange).toHaveBeenLastCalledWith({
    concise: { channel_id: 101, model: 'fast' },
    auto: { channel_id: 101, model: 'balanced' },
    detailed: { channel_id: 102, model: 'alternate' },
  })
  await user.click(
    screen.getByRole('combobox', { name: 'Summary channel (Concise)' })
  )
  await user.click(
    await screen.findByRole('option', { name: 'Secondary (#102)' })
  )
  expect(onChange).toHaveBeenLastCalledWith({
    concise: { channel_id: 102, model: '' },
    auto: { channel_id: 101, model: 'balanced' },
    detailed: { channel_id: 102, model: 'alternate' },
  })
  client.clear()
})
