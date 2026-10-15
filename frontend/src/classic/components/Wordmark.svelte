<script lang="ts">
  // Ali's logo (assets/logo.png), about 28 px tall in the header
  // (file 02, section 2: "Logo"). The source file lives at assets/logo.png
  // in the repository; this copy is bundled with the frontend.
  // v1.5.0: a size prop — the About page shows it a step larger (32 px)
  // and FirstRun larger still, without style overrides from outside.
  interface Props {
    size?: number;
  }
  let { size = 28 }: Props = $props();
</script>

<img
  src={import.meta.env.BASE_URL + "logo.png"}
  alt="Younblock"
  class="wordmark"
  style="height: {size}px"
  draggable="false"
/>

<style>
  .wordmark {
    width: auto;
    display: block;
    user-select: none;
  }
</style>
