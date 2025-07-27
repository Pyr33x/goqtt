# GoQTT

GoQTT is a lightweight and high-performance MQTT 3.1.1 broker implemented in Go. It’s designed with modularity, speed, and developer experience in mind — perfect for learning, hacking, or deploying minimal brokers in custom environments.

[![Workflows](https://img.shields.io/github/actions/workflow/status/pyr33x/goqtt/.github%2Fworkflows%2Ftest.yml?color=40BA12)](https://github.com/pyr33x/goqtt/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/pyr33x/goqtt)](https://goreportcard.com/report/github.com/pyr33x/goqtt)
[![Git Tag](https://img.shields.io/github/v/tag/pyr33x/goqtt?color=40BA12)](https://github.com/pyr33x/goqtt/tags)
[![License](https://img.shields.io/github/license/pyr33x/goqtt?color=40BA12)](https://github.com/pyr33x/goqtt/blob/master/LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/pyr33x/goqtt.svg)](https://pkg.go.dev/github.com/pyr33x/goqtt)

---

## Features

- ✅ MQTT 3.1.1 compliant (⚠️ under development)
- ⚡ Lightweight and high-performance
- 🔨 Modular and extensible architecture
- ⚙️ In-memory session store

---

## Getting Started

### Clone
```bash
git clone https://github.com/pyr33x/goqtt.git && cd goqtt
```

### Install Dependencies
```bash
go mod download
```

### Run
```bash
make all
```

## License
GoQTT is licensed under the [MIT License](https://github.com/Pyr33x/goqtt/blob/master/LICENSE).
