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
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { PricingSidebar } from '../components/pricing-sidebar'
import {
  PricingToolbar,
  type PricingToolbarProps,
} from '../components/pricing-toolbar'
import { filterByCapabilities, filterByQuotaType } from '../lib/filters'
import type { PricingModel } from '../types'

function toolbarProps(): PricingToolbarProps {
  return {
    filteredCount: 2,
    totalCount: 2,
    sortBy: 'name',
    tokenUnit: 'M',
    showRechargePrice: false,
    viewMode: 'card',
    quotaTypeFilter: 'all',
    endpointTypeFilter: 'all',
    vendorFilter: 'all',
    groupFilter: 'all',
    tagFilter: 'all',
    capabilityFilters: {
      modelType: 'all',
      inputModalities: [],
      outputModalities: [],
      minContext: 0,
    },
    onCapabilityFiltersChange: vi.fn(),
    onSortChange: vi.fn(),
    onTokenUnitChange: vi.fn(),
    onRechargePriceChange: vi.fn(),
    onViewModeChange: vi.fn(),
    onQuotaTypeChange: vi.fn(),
    onEndpointTypeChange: vi.fn(),
    onVendorChange: vi.fn(),
    onGroupChange: vi.fn(),
    onTagChange: vi.fn(),
    vendors: [],
    groups: ['default', 'premium'],
    groupRatios: { default: 1, premium: 3 },
    tags: [],
    models: [],
    hasActiveFilters: false,
    activeFilterCount: 0,
    onClearFilters: vi.fn(),
  }
}

describe('pricing controls', () => {
  it('combines model type, input/output modalities and minimum context without including unconfigured models', () => {
    const base = {
      id: 1,
      model_name: 'multimodal',
      model_ratio: 1,
      completion_ratio: 1,
      quota_type: 0,
      enable_groups: ['default'],
    }
    const models: PricingModel[] = [
      {
        ...base,
        model_type: 'chat',
        input_modalities: ['text', 'image'],
        output_modalities: ['text', 'audio'],
        context_length: 131072,
      },
      {
        ...base,
        id: 2,
        model_name: 'text-only',
        model_type: 'chat',
        input_modalities: ['text'],
        output_modalities: ['text'],
        context_length: 131072,
      },
      {
        ...base,
        id: 3,
        model_name: 'embedding',
        model_type: 'embedding',
        input_modalities: ['text'],
        output_modalities: ['embedding'],
        context_length: 131072,
      },
      { ...base, id: 4, model_name: 'unconfigured' },
      {
        ...base,
        id: 5,
        model_name: 'text-output-only',
        model_type: 'chat',
        input_modalities: ['text', 'image'],
        output_modalities: ['text'],
        context_length: 131072,
      },
    ]
    const filters = {
      modelType: 'chat',
      inputModalities: ['text', 'image'],
      outputModalities: ['text', 'audio'],
      minContext: 131072,
    }
    expect(
      filterByCapabilities(models, filters).map((model) => model.model_name)
    ).toEqual(['multimodal'])
    expect(
      filterByCapabilities(models, { ...filters, minContext: 262144 })
    ).toEqual([])
    expect(
      filterByCapabilities(models, {
        modelType: 'all',
        inputModalities: [],
        outputModalities: [],
        minContext: 0,
      })
    ).toEqual(models)
  })

  it('selects multiple modality icons independently and lets users deselect them', async () => {
    const user = userEvent.setup()
    const props = toolbarProps()
    const { rerender } = render(<PricingSidebar {...props} />)
    const inputText = screen.getByRole('button', { name: 'Input: Text' })
    await user.click(inputText)
    expect(props.onCapabilityFiltersChange).toHaveBeenLastCalledWith({
      inputModalities: ['text'],
    })
    props.capabilityFilters = {
      ...props.capabilityFilters,
      inputModalities: ['text'],
    }
    rerender(<PricingSidebar {...props} />)
    expect(inputText).toHaveAttribute('aria-pressed', 'true')
    await user.click(screen.getByRole('button', { name: 'Input: Image' }))
    expect(props.onCapabilityFiltersChange).toHaveBeenLastCalledWith({
      inputModalities: ['text', 'image'],
    })
    props.capabilityFilters = {
      ...props.capabilityFilters,
      inputModalities: ['text', 'image'],
      outputModalities: ['text'],
    }
    rerender(<PricingSidebar {...props} />)
    expect(
      screen.getByRole('button', { name: 'Input: Image' })
    ).toHaveAttribute('aria-pressed', 'true')
    expect(screen.queryByRole('button', { name: 'Input: Decision' })).toBeNull()
    await user.click(screen.getByRole('button', { name: 'Output: Decision' }))
    expect(props.onCapabilityFiltersChange).toHaveBeenLastCalledWith({
      outputModalities: ['text', 'decision'],
    })
    await user.click(inputText)
    expect(props.onCapabilityFiltersChange).toHaveBeenLastCalledWith({
      inputModalities: ['image'],
    })
  })

  it('lets users filter by structured capabilities from the sidebar', async () => {
    const user = userEvent.setup()
    const props = toolbarProps()
    render(<PricingSidebar {...props} />)
    await user.click(screen.getByRole('button', { name: '≥ 128K' }))
    expect(props.onCapabilityFiltersChange).toHaveBeenLastCalledWith({
      minContext: 131072,
    })
    const section = screen
      .getByText('Model type')
      .closest('[data-slot="collapsible"]')
    expect(section).not.toBeNull()
    await user.click(
      within(section as HTMLElement).getByRole('button', {
        name: /^Embeddings/,
      })
    )
    expect(props.onCapabilityFiltersChange).toHaveBeenLastCalledWith({
      modelType: 'embedding',
    })
  })

  it('lets each modality section collapse without clearing its selections', async () => {
    const user = userEvent.setup()
    const props = toolbarProps()
    props.capabilityFilters.inputModalities = ['image']
    props.capabilityFilters.outputModalities = ['decision']
    render(<PricingSidebar {...props} />)

    const input = screen.getByRole('button', { name: 'Input' })
    const output = screen.getByRole('button', { name: 'Output' })
    await user.click(input)
    expect(input).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByRole('button', { name: 'Input: Image' })).toBeNull()
    expect(
      screen.getByRole('button', { name: 'Output: Decision' })
    ).toBeVisible()

    await user.click(output)
    await user.click(input)
    expect(output).toHaveAttribute('aria-expanded', 'false')
    expect(
      screen.getByRole('button', { name: 'Input: Image' })
    ).toHaveAttribute('aria-pressed', 'true')
    await user.click(output)
    expect(
      screen.getByRole('button', { name: 'Output: Decision' })
    ).toHaveAttribute('aria-pressed', 'true')
    expect(props.onCapabilityFiltersChange).not.toHaveBeenCalled()
  })

  it.each<{
    name: string
    pricing: Partial<PricingModel>
    matches: string[]
  }>([
    { name: 'legacy token pricing', pricing: {}, matches: ['token'] },
    {
      name: 'legacy per-request pricing',
      pricing: { quota_type: 1, model_price: 0.01 },
      matches: ['request'],
    },
    {
      name: 'dynamic fixed pricing with a legacy token quota type',
      pricing: {
        billing_mode: 'tiered_expr',
        billing_expr: 'tier("request", fixed(0.01))',
      },
      matches: ['request'],
    },
    {
      name: 'free dynamic per-request pricing',
      pricing: {
        billing_mode: 'tiered_expr',
        billing_expr: 'v1:tier("free", fixed(0))',
      },
      matches: ['request'],
    },
    {
      name: 'request-dependent fixed tiers with an extra multiplier',
      pricing: {
        billing_mode: 'tiered_expr',
        billing_expr:
          '(param("quality") == "hd" ? tier("hd", fixed(0.02)) : tier("base", fixed(0.01))) * (header("x-fast") == "true" ? 2 : 1)',
      },
      matches: ['request'],
    },
    {
      name: 'mixed pricing across nested time tiers classified only as token-based',
      pricing: {
        billing_mode: 'tiered_expr',
        billing_expr:
          'hour("UTC") < 12 ? (len < 1000 ? tier("short", fixed(0.01)) : tier("long", p * 2 + c * 8)) : tier("night", fixed(0.02))',
      },
      matches: ['token'],
    },
    {
      name: 'mixed pricing overriding a legacy request quota type',
      pricing: {
        quota_type: 1,
        billing_mode: 'tiered_expr',
        billing_expr:
          'len < 1000 ? tier("short", fixed(0.01)) : tier("long", p * 2 + c * 8)',
      },
      matches: ['token'],
    },
    {
      name: 'dynamic token pricing overriding a legacy request quota type',
      pricing: {
        quota_type: 1,
        billing_mode: 'tiered_expr',
        billing_expr: 'tier("base", p * 2 + c * 8)',
      },
      matches: ['token'],
    },
    {
      name: 'token pricing with fixed syntax only in its tier label',
      pricing: {
        billing_mode: 'tiered_expr',
        billing_expr: 'tier("fixed(0.01)", p * 2)',
      },
      matches: ['token'],
    },
    {
      name: 'task usage retaining its separate category',
      pricing: {
        billing_mode: 'tiered_expr',
        billing_expr: 'tier("base", u("seconds") * 0.4)',
        billing_usage_schema: { seconds: { type: 'number', unit: 'second' } },
      },
      matches: ['task'],
    },
    {
      name: 'a plugin model whose ordinary requests pay a token ratio',
      pricing: {
        billing_usage_schema: { seconds: { type: 'number', unit: 'second' } },
        ordinary_pricing: true,
        billing_plugin_variants: [
          {
            plugin_key: 'alpha',
            plugin_name: 'Alpha',
            billing_mode: 'tiered_expr',
            billing_expr: '',
            billing_usage_schema: {
              seconds: { type: 'number', unit: 'second' },
            },
          },
        ],
      },
      matches: ['token'],
    },
    {
      name: 'a plugin model without ordinary requests retaining the task category',
      pricing: {
        billing_usage_schema: { seconds: { type: 'number', unit: 'second' } },
        billing_plugin_variants: [
          {
            plugin_key: 'alpha',
            plugin_name: 'Alpha',
            billing_mode: 'tiered_expr',
            billing_expr: '',
            billing_usage_schema: {
              seconds: { type: 'number', unit: 'second' },
            },
          },
        ],
      },
      matches: ['task'],
    },
    {
      name: 'unparseable dynamic pricing retaining its legacy category',
      pricing: {
        quota_type: 1,
        billing_mode: 'tiered_expr',
        billing_expr: 'tier("request", fixed(',
      },
      matches: ['request'],
    },
  ])(
    'keeps filter results and sidebar counts consistent for $name',
    ({ pricing, matches }) => {
      const model: PricingModel = {
        id: 1,
        model_name: 'pricing-model',
        quota_type: 0,
        model_ratio: 1,
        completion_ratio: 1,
        enable_groups: ['default'],
        ...pricing,
      }
      const models = [model]
      render(<PricingSidebar {...toolbarProps()} models={models} />)

      for (const [quotaType, label] of [
        ['token', 'Token-based'],
        ['request', 'Per Request'],
        ['task', 'Task billing'],
      ]) {
        const included = matches.includes(quotaType)
        expect(filterByQuotaType(models, quotaType)).toEqual(
          included ? [model] : []
        )
        expect(
          screen.getByRole('button', {
            name: new RegExp(`^${label}\\s*${included ? 1 : 0}$`),
          })
        ).toBeVisible()
      }
      expect(filterByQuotaType(models, 'all')).toEqual([model])
      expect(
        screen.getByRole('button', { name: /^All Models\s*1$/ })
      ).toBeVisible()
    }
  )

  it('counts each model once per filter and updates counts when the catalog changes', () => {
    const props = toolbarProps()
    const base: PricingModel = {
      id: 1,
      model_name: 'text-model',
      vendor_name: 'Vendor A',
      quota_type: 0,
      model_ratio: 1,
      completion_ratio: 1,
      enable_groups: ['default'],
      tags: 'Chat,chat',
      supported_endpoint_types: ['openai', 'openai'],
    }
    const models: PricingModel[] = [
      base,
      {
        ...base,
        id: 2,
        model_name: 'image-model',
        quota_type: 1,
        tags: 'chat,Image',
        supported_endpoint_types: ['image-generation'],
      },
      {
        ...base,
        id: 3,
        model_name: 'task-model',
        vendor_name: 'Vendor B',
        tags: 'Video',
        supported_endpoint_types: ['openai-video'],
        billing_usage_schema: { seconds: { type: 'number', unit: 'second' } },
      },
    ]
    const sidebarProps = {
      ...props,
      vendors: [
        { id: 1, name: 'Vendor A' },
        { id: 2, name: 'Vendor B' },
      ],
      tags: ['Chat', 'Image', 'Video'],
    }
    const { rerender } = render(
      <PricingSidebar {...sidebarProps} models={models} />
    )

    expect(
      screen.getByRole('button', { name: /^All Vendors\s*3$/ })
    ).toBeVisible()
    expect(screen.getByRole('button', { name: /^Vendor A\s*2$/ })).toBeVisible()
    expect(screen.getByRole('button', { name: /^Vendor B\s*1$/ })).toBeVisible()
    expect(screen.getByRole('button', { name: /^Chat\s*2$/ })).toBeVisible()
    expect(screen.getByRole('button', { name: /^Chat\s*1$/ })).toBeVisible()
    expect(
      screen.getByRole('button', { name: /^Token-based\s*1$/ })
    ).toBeVisible()
    expect(
      screen.getByRole('button', { name: /^Per Request\s*1$/ })
    ).toBeVisible()
    expect(
      screen.getByRole('button', { name: /^Task billing\s*1$/ })
    ).toBeVisible()

    rerender(<PricingSidebar {...sidebarProps} models={[models[1]]} />)
    expect(
      screen.getByRole('button', { name: /^All Vendors\s*1$/ })
    ).toBeVisible()
    expect(screen.getByRole('button', { name: /^Vendor A\s*1$/ })).toBeVisible()
    expect(screen.queryByRole('button', { name: /^Vendor B\s*1$/ })).toBeNull()
    expect(screen.getByRole('button', { name: /^Chat\s*1$/ })).toBeVisible()
    expect(screen.getByRole('button', { name: /^Chat\s*0$/ })).toBeVisible()
    expect(
      screen.getByRole('button', { name: /^Token-based\s*0$/ })
    ).toBeVisible()
    expect(
      screen.getByRole('button', { name: /^Task billing\s*0$/ })
    ).toBeVisible()
  })

  it('changes the token unit and keeps the selected unit pressed when clicked again', async () => {
    const props = toolbarProps()
    const user = userEvent.setup()
    const { rerender } = render(<PricingToolbar {...props} />)
    await user.click(screen.getByRole('button', { name: '/1K' }))
    expect(props.onTokenUnitChange).toHaveBeenCalledWith('K')
    rerender(<PricingToolbar {...props} tokenUnit='K' />)
    const selected = screen.getByRole('button', { name: '/1K' })
    expect(selected).toHaveAttribute('aria-pressed', 'true')
    await user.click(selected)
    expect(selected).toHaveAttribute('aria-pressed', 'true')
    expect(props.onTokenUnitChange).toHaveBeenCalledTimes(1)
  })

  it('changes the recharge display mode with an accessible selected state', async () => {
    const props = toolbarProps()
    const user = userEvent.setup()
    const { rerender } = render(<PricingToolbar {...props} />)
    await user.click(screen.getByRole('button', { name: 'Recharge' }))
    expect(props.onRechargePriceChange).toHaveBeenCalledWith(true)
    rerender(<PricingToolbar {...props} showRechargePrice />)
    expect(screen.getByRole('button', { name: 'Recharge' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    expect(screen.getByRole('button', { name: 'Standard' })).toHaveAttribute(
      'aria-pressed',
      'false'
    )
  })

  it('switches to table view with the keyboard and exposes the selected view', async () => {
    const props = toolbarProps()
    const user = userEvent.setup()
    const { rerender } = render(<PricingToolbar {...props} />)
    const tableButton = screen.getByRole('button', { name: 'Table view' })
    tableButton.focus()
    await user.keyboard('{Enter}')
    expect(props.onViewModeChange).toHaveBeenCalledWith('table')
    rerender(<PricingToolbar {...props} viewMode='table' />)
    expect(tableButton).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Card view' })).toHaveAttribute(
      'aria-pressed',
      'false'
    )
  })

  it('selects price sorting from the shared dropdown', async () => {
    const props = toolbarProps()
    const user = userEvent.setup()
    render(<PricingToolbar {...props} />)
    await user.click(screen.getByRole('button', { name: 'Name' }))
    await user.click(
      screen.getByRole('menuitem', { name: 'Input Price: Low to High' })
    )
    expect(props.onSortChange).toHaveBeenCalledWith('input-price-low')
  })

  it('opens mobile filters from the left, selects a group, and restores focus on close', async () => {
    const props = toolbarProps()
    const user = userEvent.setup()
    const { rerender } = render(<PricingToolbar {...props} />)
    await user.click(screen.getByRole('button', { name: 'Filter' }))
    const dialog = await screen.findByRole('dialog', { name: 'Filter' })
    expect(dialog).toHaveAttribute('data-side', 'left')
    expect(within(dialog).getByRole('button', { name: 'Reset' })).toBeDisabled()
    await user.click(within(dialog).getByRole('button', { name: /premium/ }))
    expect(props.onGroupChange).toHaveBeenCalledWith('premium')
    rerender(
      <PricingToolbar
        {...props}
        groupFilter='premium'
        hasActiveFilters
        activeFilterCount={1}
      />
    )
    expect(
      within(dialog).getByRole('button', { name: /premium/ })
    ).toHaveAttribute('aria-pressed', 'true')
    await user.click(within(dialog).getByRole('button', { name: 'Reset' }))
    expect(props.onClearFilters).toHaveBeenCalledOnce()
    await user.keyboard('{Escape}')
    expect(await screen.findByRole('button', { name: /Filter/ })).toHaveFocus()
  })
})
