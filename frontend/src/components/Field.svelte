<script lang="ts">
  import type { Snippet } from 'svelte'
  import type { FormState } from '../lib/form.svelte'

  // A labelled form field. With an error the label and input turn red and the
  // message replaces the hint underneath.
  let {
    label,
    id,
    form,
    name,
    error,
    hint,
    class: className = '',
    children
  }: {
    label?: string
    id?: string
    // the error comes from form.error(name), or directly from `error`
    form?: FormState
    name?: string
    error?: string
    hint?: string | Snippet
    class?: string
    children: Snippet
  } = $props()

  let message = $derived(error || (form && name ? form.error(name) : ''))

  function edited() {
    if (form && name) form.clear(name)
  }
</script>

<div class="{message ? 'field-invalid' : ''} {className}" oninput={edited} onchange={edited}>
  {#if label}
    {#if id}<label class="label" for={id}>{label}</label>{:else}<span class="label">{label}</span>{/if}
  {/if}
  {@render children()}
  {#if message}
    <p class="field-error">{message}</p>
  {:else if typeof hint === 'string'}
    <p class="field-hint">{hint}</p>
  {:else if hint}
    <p class="field-hint">{@render hint()}</p>
  {/if}
</div>
