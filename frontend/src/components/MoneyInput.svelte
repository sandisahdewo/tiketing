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

  // The input's text is owned by this component and written straight to the DOM:
  // no reactive `value` attribute, and the DOM is only touched when the formatted
  // text or caret actually differs from what the browser already shows.
  let el: HTMLInputElement
  let previous = value != null ? formatMoney(value) : ''
  let invalid = $state(false)

  $effect(() => {
    el.value = previous
    if (autofocus) el.focus()
  })

  function commit(text: string) {
    previous = text
    const parsed = text === '' ? null : parseMoney(text)
    invalid = text !== '' && parsed === null
    if (parsed !== value) value = parsed
  }

  export function set(sen: number) {
    const text = formatMoney(sen)
    if (el.value !== text) el.value = text
    commit(text)
  }

  function apply(raw: string, caret: number) {
    const r = formatTyping(raw, caret)
    if (el.value !== r.text) el.value = r.text
    if (el.selectionStart !== r.caret || el.selectionEnd !== r.caret) el.setSelectionRange(r.caret, r.caret)
    commit(r.text)
  }

  function oninput(e: Event) {
    const ev = e as InputEvent
    if (ev.isComposing) return
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

  function oncompositionend() {
    apply(el.value, el.selectionStart ?? el.value.length)
  }

  function onblur() {
    if (previous.endsWith(',')) {
      el.value = previous.slice(0, -1)
      commit(el.value)
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
    autocorrect="off"
    spellcheck="false"
    {placeholder}
    {oninput}
    {oncompositionend}
    {onblur}
    {onkeydown}
  />
</div>
{#if invalid}
  <p class="mt-1 text-xs text-red-600">Angka terlalu besar</p>
{/if}
