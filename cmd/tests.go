package cmd

// ServeOptionsReset resets serve options provided on command line.
// This should be used between two tests.
func ServeOptionsReset() {
	ServeOptions = serveOptions{}
}
