<script lang="ts">
  import type { receipt } from '../../wailsjs/go/models'
  import { session } from '../lib/session.svelte'

  let { lines, logo = '' }: { lines: receipt.Line[]; logo?: string } = $props()

  let cols = $derived(session.info?.paperWidth === 80 ? 48 : 32)
  let dots = $derived(session.info?.paperWidth === 80 ? 576 : 384)
  let logoPx = $state(0)
</script>

<div class="bg-white px-4 py-5 font-mono text-[13px] leading-snug whitespace-pre text-black shadow" style="width: calc({cols}ch + 2rem)">
  {#each lines as l}
    {#if l.logo}
      {#if logo}
        <img
          src={logo}
          alt="Logo"
          class="mx-auto mb-1 block [image-rendering:pixelated]"
          style={logoPx ? `width: calc(${cols}ch * ${logoPx / dots})` : ''}
          onload={(e) => (logoPx = (e.currentTarget as HTMLImageElement).naturalWidth)}
        />
      {/if}
    {:else if l.size === 'double'}
      <div class="text-[26px] leading-tight {l.bold ? 'font-bold' : ''}">{l.text || ' '}</div>
    {:else if l.size === 'tall'}
      <div class="h-[2.6em] {l.bold ? 'font-bold' : ''}"><span class="inline-block origin-top-left scale-y-200">{l.text}</span></div>
    {:else}
      <div class={l.bold ? 'font-bold' : ''}>{l.text || ' '}</div>
    {/if}
  {/each}
</div>
