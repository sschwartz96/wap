<script lang="ts">
  import { count } from '../components/store'
  let world = 'world'

  const countUp = () => $count++
</script>

<div>Hello {world}!</div>
<button on:click={() => countUp()}>Please click me!</button>
<span>Count: {$count}</span>

<style>
  div {
    background: black;
    color: white;
  }
</style>
