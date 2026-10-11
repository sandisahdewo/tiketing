<script lang="ts">
  import { formatMoney, formatTyping, parseMoney } from '../lib/money'

  let {
    value = $bindable<number | null>(null),
    id,
    placeholder = '0',
    autofocus = false,
    large = false,
    onenter
  }: {
    value?: number | null
    id?: string
    placeholder?: string
    autofocus?: boolean
    large?: boolean
    onenter?: () => void
  } = $props()

  const initial = value != null ? formatMoney(value) : ''
  let text = $state(initial)
  let invalid = $derived(text.trim() !== '' && parseMoney(text) === null)
  let el: HTMLInputElement
  let previous = initial

  $effect(() => {
    if (autofocus) el?.focus()
  })

  export function set(sen: number) {
    value = sen
    text = previous = formatMoney(sen)
  }

  function apply(raw: string, caret: number) {
    const r = formatTyping(raw, caret)
    text = previous = el.value = r.text
    el.setSelectionRange(r.caret, r.caret)
    value = r.text === '' ? null : parseMoney(r.text)
  }

  function oninput(e: Event) {
    const ev = e as InputEvent
    let raw = el.value
    let caret = el.selectionStart ?? raw.length

    if (ev.inputType === 'insertFromPaste') {
      const pasted = parseMoney(raw)
      if (pasted !== null) {
        const formatted = formatMoney(pasted)
        apply(formatted, formatted.length)
        return
      }
    }
    if (ev.inputType === 'insertText' && ev.data === '.' && !previous.includes(',')) {
      raw = raw.slice(0, caret - 1) + ',' + raw.slice(caret)
    }
    if (ev.inputType === 'deleteContentBackward' && previous[caret] === '.' && raw.length === previous.length - 1) {
      raw = raw.slice(0, caret - 1) + raw.slice(caret)
      caret -= 1
    }
    if (ev.inputType === 'deleteContentForward' && previous[caret] === '.' && raw.length === previous.length - 1) {
      raw = raw.slice(0, caret) + raw.slice(caret + 1)
    }
    apply(raw, caret)
  }

  function onblur() {
    if (text.endsWith(',')) {
      text = previous = text.slice(0, -1)
      value = text === '' ? null : parseMoney(text)
    }
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') onenter?.()
  }
</script>

<div class="relative">
  <span class="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-slate-400 {large ? 'text-xl' : 'text-sm'}">Rp</span>
  <input
    bind:this={el}
    {id}
    class="input text-right {large ? 'py-3 pl-12 text-2xl font-semibold' : 'pl-10'} {invalid ? 'border-red-400 focus:border-red-500 focus:ring-red-100' : ''}"
    inputmode="decimal"
    autocomplete="off"
    {placeholder}
    value={text}
    {oninput}
    {onblur}
    {onkeydown}
  />
</div>
{#if invalid}
  <p class="mt-1 text-xs text-red-600">Angka terlalu besar</p>
{/if}
