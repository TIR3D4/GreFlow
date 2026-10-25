package app

import "embed"

//go:embed index.html app.js styles.css assets/*
var Files embed.FS
