# Jasima

Capstone II project for IMA at NYU Shanghai.

Jasima is Toki Pona for "reflect", "echo", "mirror", or "duplicate". Similarly, the agents in this simulation reflect reality.

## Todo

- \[\] Implement Server Side Events
  - See <https://medium.com/@rian.eka.cahya/server-sent-event-sse-with-go-10592d9c2aa1>
- \[\] Develop website view with SvelteKit
- \[\] Add GPT system instructions
- \[\] Unit testing for evolution function

## Getting Started

There are a few required system dependencies needed regardless of whether you use the provided `flake.nix` file to setup the development environment.

- Docker

### No `nix`

Even though it is highly recommended to have nix installed on your system, it is not needed. Here are the required dependencies, for development:

- Go >= 1.24
- protobuf
- protoc-gen-go
- protoc-gen-go-grpc
- go-task

Install them with your package manager of choice. Then install the dependencies:

```bash
go mod vendor
go mod tidy
```

### Nix ⭐

If you have the [nix package manager](https://nixos.org/) installed and have _nix experimental flakes_ **enabled**, copy and paste these commands into your terminal:

```bash
# Initialize development environment.
nix develop .
```

This command will also install dependencies.

For convenience, a `.envrc` file exists for this repository, so if you have `direnv` installed, you can allow this repository to auto-load the nix environment with `direnv allow`. Otherwise, to reinitialize the development environment every time you enter this repository, make sure to re-run:

```bash
nix develop .
```

## Useful Links and Sources

- [Toki Pona Dictionary](https://nimi.li/)
- [Toki Pona Sitelen SVGs](https://drive.google.com/open?id=1JnoEV7DFaZBbAZLaL1MXrqlVGm99onnP)
- [Sitelen Pona](https://en.wikipedia.org/wiki/Sitelen_Pona)
- <https://en.wikipedia.org/wiki/Sitelen_Pona#cite_ref-21>
- [Free SVG Repo](https://www.svgrepo.com/)

### Nix Stuff

- <https://github.com/cachix/git-hooks.nix>
- <https://bmcgee.ie/posts/2023/11/nix-my-workflow/>

### gRPC

- <https://protobuf.dev/overview/>
- <https://github.com/grpc/grpc-go/blob/master/examples/route_guide/routeguide/route_guide.pb.go>
