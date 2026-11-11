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
import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { useSystemConfigStore } from '@/stores/system-config-store'

import { RankingArena } from '../components/ranking-arena'
import type { RankingsSnapshot } from '../types'

vi.mock('@visactor/react-vchart', () => ({ VChart: () => null }))

const snapshot: RankingsSnapshot = {
  models: [
    {
      rank: 1,
      model_name: 'gpt-5',
      vendor: 'OpenAI',
      category: 'all',
      total_tokens: 1000,
      share: 0.8,
      growth_pct: 25,
    },
    {
      rank: 2,
      model_name: 'claude',
      vendor: 'Anthropic',
      category: 'all',
      total_tokens: 250,
      share: 0.2,
      growth_pct: -10,
    },
  ],
  vendors: [
    {
      rank: 1,
      vendor: 'OpenAI',
      total_tokens: 1000,
      share: 0.8,
      growth_pct: 25,
      models_count: 1,
      top_model: 'gpt-5',
    },
    {
      rank: 2,
      vendor: 'Anthropic',
      total_tokens: 250,
      share: 0.2,
      growth_pct: -10,
      models_count: 1,
      top_model: 'claude',
    },
  ],
  top_movers: [],
  top_droppers: [],
  models_history: {
    points: [
      {
        ts: '2026-11-01T00:00:00Z',
        label: '00:00',
        model: 'gpt-5',
        vendor: 'OpenAI',
        tokens: 1000,
        requests: 10,
      },
      {
        ts: '2026-11-01T00:00:00Z',
        label: '00:00',
        model: 'claude',
        vendor: 'Anthropic',
        tokens: 250,
        requests: 5,
      },
    ],
    models: [
      { name: 'gpt-5', vendor: 'OpenAI', total: 1000 },
      { name: 'claude', vendor: 'Anthropic', total: 250 },
    ],
    buckets: 1,
  },
  vendor_share_history: { points: [], vendors: [], buckets: 0 },
}
let client: QueryClient
beforeEach(() => {
  useSystemConfigStore.getState().setConfig({
    modelCustomizations: {
      'gpt-5': {
        display_name: 'My favorite',
        icon: '',
      },
    },
  })
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  client.setQueryData(['status'], {
    model_customizations: {
      'gpt-5': {
        display_name: 'My favorite',
        icon: '',
      },
    },
  })
})
afterEach(() => {
  client.clear()
  useSystemConfigStore.getState().setConfig({ modelCustomizations: {} })
})

async function renderArena(): Promise<void> {
  const root = createRootRoute({
    component: () => <RankingArena snapshot={snapshot} />,
  })
  const router = createRouter({
    routeTree: root,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  await router.load()
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  await screen.findByRole('searchbox')
}

it('finds models by a custom display name and shows an empty state for unmatched searches', async () => {
  await renderArena()
  fireEvent.change(screen.getByRole('searchbox'), {
    target: { value: 'FAVORITE' },
  })
  expect(screen.getAllByRole('link', { name: /My favorite/ })).toHaveLength(2)
  fireEvent.change(screen.getByRole('searchbox'), {
    target: { value: 'missing' },
  })
  expect(await screen.findByText('No models found')).toBeInTheDocument()
})

it('filters the model list by vendor and switches to vendor rankings', async () => {
  await renderArena()
  fireEvent.click(screen.getByRole('button', { name: 'Anthropic' }))
  expect(screen.getByRole('button', { name: 'Anthropic' })).toHaveAttribute(
    'aria-pressed',
    'true'
  )
  expect(screen.queryByText('My favorite')).not.toBeInTheDocument()
  expect(screen.getByRole('group', { name: 'Models' })).toHaveTextContent(
    'claude'
  )
  fireEvent.click(screen.getByRole('tab', { name: 'Vendors' }))
  expect(await screen.findByRole('tab', { name: 'Vendors' })).toHaveAttribute(
    'aria-selected',
    'true'
  )
})
