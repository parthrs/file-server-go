<script>
  export let data;
</script>

<ul>
  {#each data.data.split('\n') as file}
    <li>{file}</li>
  {/each}
</ul>

<!-- <p>{data.data}</p> -->