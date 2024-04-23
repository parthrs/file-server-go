<script>
  import Box from './Box.svelte';
  import Hoverable from './Hoverable.svelte';
  export let data;
</script>
<h2>A simple file server written in Go and Svelte</h2>
<p>Upload and download files from the respective boxes below!</p>
<Box>
  <h3>Download a file</h3>
  <ul>
    {#each data.data.split('\n') as file}
      <Hoverable let:hovering={active}>
        <div class:active>
          {#if active}
          <li><a href="http://bkend-svc.default.svc.cluster.local:37899/download/{file}"><b>{file}</b></a></li>
          {:else}
          <li><a href="http://bkend-svc.default.svc.cluster.local:37899/download/{file}">{file}</a></li>
          {/if}
        </div>
      </Hoverable>
    {/each}
  </ul>
</Box>


<!-- <label for="many">Upload multiple files of any type:</label> -->
<!-- <input bind:files id="fileUpload" multiple type="file" /> -->

<Box>
  <h3>Upload a file</h3>
  <form method="post" enctype="multipart/form-data">
    <div class="group">
      <input
        type="file"
        id="file"
        name="fileToUpload"
        required
      />
    </div>
    <br>
    <button type="submit">Submit</button>
  </form>
</Box>