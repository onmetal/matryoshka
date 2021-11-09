# matryoshka

[![Go Report Card](https://goreportcard.com/badge/github.com/onmetal/matryoshka)](https://goreportcard.com/report/github.com/onmetal/matryoshka)
[![Go Reference](https://pkg.go.dev/badge/github.com/onmetal/matryoshka.svg)](https://pkg.go.dev/github.com/onmetal/matryoshka)
[![Build and Publish Docker Image](https://github.com/onmetal/matryoshka/actions/workflows/publish-docker.yml/badge.svg)](https://github.com/onmetal/matroyshka/actions/workflows/publish-docker.yml)

## Overview

This projects contains controllers to host components / configuration of
Kubernetes within Kubernetes, hence the name 'Matryoshka', referring to
the Russian stacking dolls.

## Installation, Usage and Development

matryoshka is a [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) project.
The API definitions can be found at [apis](apis).

The project also comes with a well-defined [Makefile](Makefile).
The CRDs can be deployed using

```shell
make install
```

To run the controllers locally, just run

```shell
make run
```

To deploy the controllers into the currently selected (determined by your current kubeconfig) cluster,
just run

```shell
make deploy
```

This will apply the [default kustomization)[config/default] with correct RBAC permissions.


## Contributing

We'd love to get feedback from you. Please report bugs, suggestions or post questions by opening a GitHub issue.

## License

[Apache-2.0](LICENSE)

