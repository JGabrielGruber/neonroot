package vault

import "path/filepath"

// Images are stored in the vault under images/<name>/ as a Containerfile
// (the definition, editable and versioned) plus image.tar (the built data,
// produced by `podman save`, loaded into the tmpfs store on load — no network).

// ImageDir returns the directory holding an image's definition and data.
func ImageDir(vaultPath, name string) string {
	return filepath.Join(vaultPath, "images", name)
}

// ContainerfilePath returns the location of an image's build definition.
func ContainerfilePath(vaultPath, name string) string {
	return filepath.Join(ImageDir(vaultPath, name), "Containerfile")
}

// ImageTarPath returns the location of an image's saved data (podman save).
func ImageTarPath(vaultPath, name string) string {
	return filepath.Join(ImageDir(vaultPath, name), "image.tar")
}

// ImageRef is the local podman reference NeonRoot builds/loads an image under.
func ImageRef(name string) string {
	return "localhost/neonroot-" + name + ":latest"
}
