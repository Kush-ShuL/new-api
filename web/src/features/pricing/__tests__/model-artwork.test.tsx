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
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { ModelCard } from '../components/model-card'
import { ModelDetailsContent } from '../components/model-details'
import type { PricingModel } from '../types'

vi.mock('@visactor/react-vchart', () => ({ VChart: () => null }))

const model: PricingModel = {
  id: 1,
  model_name: 'art-model',
  description: 'Model description',
  quota_type: 0,
  model_ratio: 1,
  completion_ratio: 2,
  enable_groups: ['default'],
}
afterEach(() => {
  cleanup()
  useSystemConfigStore.getState().setConfig({ modelCustomizations: {} })
})

it('places artwork behind the card content and between the detail heading and description', () => {
  useSystemConfigStore.getState().setConfig({
    modelCustomizations: {
      'art-model': {
        display_name: 'Art model',
        icon: '',
        background: 'https://example.com/art.webp',
      },
    },
  })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: { points: [], summary: {} } },
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const view = render(
    <QueryClientProvider client={client}>
      <ModelCard model={model} onClick={() => {}} />
      <ModelDetailsContent
        model={model}
        groupRatio={{ default: 1 }}
        usableGroup={{ default: { desc: '', ratio: 1 } }}
        endpointMap={{}}
        autoGroups={[]}
        priceRate={1}
        usdExchangeRate={1}
        tokenUnit='M'
      />
    </QueryClientProvider>
  )
  const cardImage = view.container.querySelector('.model-card-background')
  expect(cardImage).toHaveAttribute('src', 'https://example.com/art.webp')
  expect(cardImage).toHaveAttribute('aria-hidden', 'true')
  expect(view.container.querySelector('.model-card-mask')).not.toBeNull()
  const detailImage = screen.getByRole('img', { name: 'Model card background' })
  expect(detailImage).toHaveAttribute('src', 'https://example.com/art.webp')
  const heading = screen.getByRole('heading', { level: 1 })
  expect(
    heading.compareDocumentPosition(detailImage) &
      Node.DOCUMENT_POSITION_FOLLOWING
  ).toBeTruthy()
  expect(
    detailImage.compareDocumentPosition(
      screen.getAllByText('Model description').at(-1) as Node
    ) & Node.DOCUMENT_POSITION_FOLLOWING
  ).toBeTruthy()
  client.clear()
})
