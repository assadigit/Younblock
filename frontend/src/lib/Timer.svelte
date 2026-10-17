<script lang="ts">
  // Timer.svelte — the big connection timer (design.md: 40/700, and every
  // digit sits in a fixed-width box so the numbers never jitter).
  export let value = "00:00:00";

  $: chars = [...value];
</script>

<div class="timer" role="timer" aria-label={value}>
  {#each chars as ch, i (i)}
    <span class={ch === ":" ? "colon" : "digit"}>{ch}</span>
  {/each}
</div>

<style>
  .timer {
    display: flex;
    justify-content: center;
    font-size: var(--fs-timer);
    font-weight: var(--fw-bold);
    color: var(--text-primary);
    line-height: 1.15;
    /* M8-j: tabular figures when the face has them; the fixed boxes
       below already guarantee steady spacing when it does not. */
    font-variant-numeric: tabular-nums;
  }
  .digit {
    width: 0.66em;
    text-align: center;
  }
  .colon {
    width: 0.4em;
    text-align: center;
    opacity: 0.72;
  }
</style>
