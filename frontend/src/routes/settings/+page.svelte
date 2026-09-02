<script>
	import SignInMethods from '$lib/SignInMethods.svelte';
	import AiConnector from '$lib/AiConnector.svelte';
</script>

<p class="eyebrow" style="margin-top:2rem">Settings</p>
<h1>Your account</h1>

<div class="column form-width">
	<SignInMethods />

	<div style="margin-top:1.6rem">
		<AiConnector />
	</div>
</div>
