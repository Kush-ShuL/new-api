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
import { fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { ModelAvailability } from '..'
import type { AvailabilityData } from '../api'
import { AvailabilityTable } from '../components/availability-table'

const hour = Date.UTC(2026, 10, 1, 0) / 1000
const data: AvailabilityData = {
  window_start: hour,
  window_end: hour + 23 * 3600 + 1800,
  models: [
    {
      model_name: 'qwen3.5:4b',
      request_count: 10,
      series: [{ ts: hour, request_count: 10, success_count: 9 }],
    },
    {
      model_name: 'deepseek-flash',
      request_count: 2,
      series: [{ ts: hour, request_count: 2, success_count: 0 }],
    },
  ],
}

let client: QueryClient
const originalConfig = useSystemConfigStore.getState().config

beforeEach(() => {
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  client.setQueryData(['status'], { model_customizations: {} })
  useSystemConfigStore.setState({
    config: { ...originalConfig, modelCustomizations: {} },
  })
})
afterEach(() => {
  client.clear()
  vi.restoreAllMocks()
  useSystemConfigStore.setState({ config: originalConfig })
})

function renderPage(): void {
  render(
    <QueryClientProvider client={client}>
      <ModelAvailability />
    </QueryClientProvider>
  )
}

describe('hourly model availability', () => {
  it('shows custom icons first, catalog icons next, and an initial for an unknown model', () => {
    useSystemConfigStore.setState({
      config: {
        ...originalConfig,
        modelCustomizations: {
          'qwen3.5:4b': {
            display_name: '',
            icon: 'https://example.com/custom-qwen.png',
          },
        },
      },
    })
    render(
      <AvailabilityTable
        data={{
          ...data,
          models: [
            { ...data.models[0], icon: 'https://example.com/qwen.png' },
            { ...data.models[1], icon: 'https://example.com/deepseek.png' },
            { model_name: 'unlisted-model', request_count: 0, series: [] },
          ],
        }}
      />
    )
    const qwen = screen.getByRole('rowheader', { name: 'qwen3.5:4b' })
    const deepseek = screen.getByRole('rowheader', { name: 'deepseek-flash' })
    expect(qwen.querySelector('img')).toHaveAttribute(
      'src',
      'https://example.com/custom-qwen.png'
    )
    expect(deepseek.querySelector('img')).toHaveAttribute(
      'src',
      'https://example.com/deepseek.png'
    )
    expect(
      within(
        screen.getByRole('rowheader', { name: 'unlisted-model' })
      ).getByText('U')
    ).toBeVisible()
  })

  it('renders a failed hour as 0% and an hour without requests as no data', () => {
    render(
      <QueryClientProvider client={client}>
        <AvailabilityTable data={data} />
      </QueryClientProvider>
    )
    const failed = screen.getByRole('row', { name: /deepseek-flash/ })
    expect(within(failed).getByText('0.0%')).toBeInTheDocument()
    expect(
      within(failed).getAllByRole('cell', { name: /No Data/ })
    ).toHaveLength(23)
    expect(
      within(failed).getByRole('cell', { name: /0 of 2 requests succeeded/ })
    ).toHaveTextContent('0.0%')
    expect(
      within(failed).getByRole('cell', { name: /0 of 2 requests succeeded/ })
    ).toHaveAttribute('data-availability-level', 'critical')
    expect(screen.getAllByRole('columnheader')).toHaveLength(25)
    expect(screen.getByRole('region')).toHaveAttribute('tabindex', '0')
  })

  it('filters model names without case sensitivity and shows an empty search result', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({ data: { success: true, data } })
    renderPage()
    await screen.findByRole('rowheader', { name: 'deepseek-flash' })
    fireEvent.change(screen.getByRole('searchbox'), {
      target: { value: ' QWEN ' },
    })
    expect(
      await screen.findByRole('rowheader', { name: 'qwen3.5:4b' })
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('rowheader', { name: 'deepseek-flash' })
    ).not.toBeInTheDocument()
    fireEvent.change(screen.getByRole('searchbox'), {
      target: { value: 'missing' },
    })
    expect(await screen.findByText('No models found')).toBeInTheDocument()
  })

  it('switches between 24, 72, and 168 hourly columns and marks the selected range', async () => {
    vi.spyOn(api, 'get').mockImplementation(async (_url, config) => ({
      data: {
        success: true,
        data: {
          ...data,
          window_start:
            hour +
            (24 -
              Number(
                (config?.params as { hours: number } | undefined)?.hours ?? 24
              )) *
              3600,
        },
      },
    }))
    renderPage()
    await screen.findByRole('table')
    expect(screen.getAllByRole('columnheader')).toHaveLength(25)
    fireEvent.click(screen.getByRole('button', { name: '72 hours' }))
    await screen.findByRole('table')
    expect(screen.getByRole('button', { name: '72 hours' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    expect(screen.getAllByRole('columnheader')).toHaveLength(73)
    fireEvent.click(screen.getByRole('button', { name: '7 days' }))
    await screen.findByRole('table')
    expect(screen.getAllByRole('columnheader')).toHaveLength(169)
  })

  it('retries a failed request and renders returned data', async () => {
    vi.spyOn(api, 'get')
      .mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValue({ data: { success: true, data } })
    renderPage()
    fireEvent.click(await screen.findByRole('button', { name: 'Retry' }))
    expect(
      await screen.findByRole('rowheader', { name: 'qwen3.5:4b' })
    ).toBeInTheDocument()
  })
})
