<script>
  /** @type {import('./$types').PageLoad} */
  export async function load({ params }) {
    return { id: params.id };
  }
</script>

<div class="max-w-3xl mx-auto">
  <p>Loading transfer {id}...</p>
</div>
