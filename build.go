package main

// conains the necessary informatino to build a Svelte script
type SvelteBuild struct {
	Name          string
	SvelteLoc     string // location where the script lies
	EntryPointLoc string // this needs to be written ./tmp/*
	BuildLoc      string // location where svelte route will be built
}

var buildScriptTmpl = `const sveltePreprocess = require('svelte-preprocess');
const esbuildSvelte = require('esbuild-svelte');
const esbuild = require('esbuild');

esbuild
  .build({
    entryPoints: ['{{.EntryPoint}}'],
    bundle: true,
    outdir: './public/build',
    plugins: [
      esbuildSvelte({
        preprocess: sveltePreprocess(),
      }),
    ],
  })
  .catch(() => process.exit(1));`

var esBuildEntryTmpl = `import {{.Name}} from '{{.SvelteLoc}}';
const app = new App({
  target: document.body,
  props: {},
});
export default app;`

// outdir: ./public/build

func build() error {
	//
	return nil
}
