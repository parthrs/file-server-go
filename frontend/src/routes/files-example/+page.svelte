<script>
  export let data;
</script>

<ul>
  {#each data.files as file}
    <li>{file}</li>
  {/each}
</ul>