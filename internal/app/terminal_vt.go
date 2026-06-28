package app

// expandStandaloneBackspaceForVT converts standalone BS bytes into a destructive
// backspace sequence understood by VT terminals such as xterm.js. Some Windows
// console hosts emit only BS while editing command lines; BS moves the cursor but
// does not clear the previous cell in a VT terminal.
func expandStandaloneBackspaceForVT(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	changed := false
	for i := 0; i < len(data); i++ {
		if data[i] != '\b' {
			continue
		}
		if i+2 < len(data) && data[i+1] == ' ' && data[i+2] == '\b' {
			i += 2
			continue
		}
		changed = true
		break
	}
	if !changed {
		return append([]byte(nil), data...)
	}
	out := make([]byte, 0, len(data)+8)
	for i := 0; i < len(data); i++ {
		if data[i] != '\b' {
			out = append(out, data[i])
			continue
		}
		if i+2 < len(data) && data[i+1] == ' ' && data[i+2] == '\b' {
			out = append(out, '\b', ' ', '\b')
			i += 2
			continue
		}
		out = append(out, '\b', ' ', '\b')
	}
	return out
}
