<script lang="ts">
  // Ali's logo (assets/logo.png), about 28 px tall in the header
  // (file 02, section 2: "Logo"). The source file lives at assets/logo.png
  // in the repository; this copy is bundled with the frontend.
</script>

<img src={import.meta.env.BASE_URL + "logo.png"} alt="Younblock" class="wordmark" draggable="false" />

<style>
  .wordmark {
    height: 28px;
    width: auto;
    display: block;
    user-select: none;
  }
</style>
