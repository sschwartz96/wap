Web
Application
Program

Why? I want to be able to write a frontend with slickness of Svelte and serve it with a Go backend which could be extending easily.
I used frameworks like Sapper but hated the idea of having to check whether or not I was in the server or client.
The clear separation here is that what you write in Svelte is always going to be run in the browser.
The routes are generated during build and served using a Go backend.
Inspired by Plenti, Sapper, SvelteKit, etc.

Here is the work flow.

Write frontend in Svelte -> API in Go => Hydrate Svelte with Go / serve JS & CSS with Go.
(or)
Write frontend in Svelte -> Connect via existing API => serve JS & CSS with Go.

wap:
	* new *name* : creates a new project with name
	* run        : starts dev server and recompiles files from 
	* build      : builds the project in a simple binary to be ran on server

\*build also generates routes to be served at their respected endpoints in ./src/routes/\*

Dependencies: 
* Node
* \* those who are in embedded/frontend/package.json & embedded/backend/go.mod

esbuild is used to "bundle", but since preproccessing is required for Typescript, Sass, etc we need Node to run the build script.
So it has the best of both worlds in terms of utilizing the speed of esbuild and the Javascript dependency management of npm.
