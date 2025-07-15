package metadata

func ReconcileAnnotations(existing map[string]string, defaults ...map[string]string) map[string]string {
	return merge(existing, defaults...)
}

func merge(baseAnnotations map[string]string, maps ...map[string]string) map[string]string {
	annotations := map[string]string{}
	if baseAnnotations != nil {
		annotations = baseAnnotations
	}

	for _, m := range maps {
		for k, v := range m {
			annotations[k] = v
		}
	}

	return annotations
}
