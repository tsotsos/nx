[![GoDoc](https://godoc.org/github.com/tsotsos/Nx?status.svg)](https://godoc.org/github.com/tsotsos/Nx) [![rcard](https://goreportcard.com/badge/github.com/tsotsos/Nx)](https://goreportcard.com/report/github.com/tsotsos/Nx) [![GolangCI](https://golangci.com/badges/github.com/tsotsos/nx.svg)](https://golangci.com/r/github.com/tsotsos/Nx) [![License](https://img.shields.io/github/license/tsotsos/nx)](https://img.shields.io/github/license/tsotsos/nx)

# Nx card go library

This is a small library for NX595 and similar family Network Cards. Since the NX doesn't provide any clean Rest API neither a documentation its currently used by interface API. This work is based on what the current web interface is using but it seems to work with NX595 with no issue so far.


# NX functions covered
- System Status
- System Triggers ( all : Bypass, Chime, Stay, Arm and Disarm)
- Zone Names
- Zone Statuses (all)
- Zone Bypassing

Also this library provide the option to import your custom named zones. 

# Install  and Usage
> go get -u [https://github.com/tsotsos/Nx](https://github.com/tsotsos/Nx)

> go install
