<script lang="ts">
  // Timer.svelte — the connection timer (the proportion round: 16/400,
  // exactly half the 32px headline it sits under, in the regular weight —
  // the state word is the hero of the connected screen, the timer a
  // quiet live figure beneath it, not a second headline). Every digit
  // sits in a fixed-width box so the numbers never jitter even where the
  // face lacks tabular figures.
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
    font-family: var(--font);
    font-size: var(--yb-fs-timer);
    font-weight: var(--fw-regular); /* the proportion round: regular, not bold */
    letter-spacing: 1px; /* scaled with the size (2px was drawn against 34px) */
    color: var(--yb-text);
    line-height: 1.15;
    /* tabular figures when the face has them; the fixed boxes below
       already guarantee steady spacing when it does not. */
    font-variant-numeric: tabular-nums;
  }
  /* Round-2 fix #2: the box must clear the font's widest digit ("0" at
     0.76em in this face), or the countdown digits crowd each other; 0.80em
     gives every digit equal, un-touching width. */
  .digit {
    width: 0.8em;
    text-align: center;
  }
  .colon {
    width: 0.4em;
    text-align: center;
    opacity: 0.72;
  }
</style>
