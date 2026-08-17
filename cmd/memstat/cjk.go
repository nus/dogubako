//go:build linux && !nocjk

package main

import _ "github.com/nus/dogubako/internal/cjkembed"

const cjkEnabled = true
