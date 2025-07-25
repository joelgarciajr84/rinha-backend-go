package sentinel

type Guardian struct {
	Dismissible bool
	Context     string
}

func (g Guardian) Error() string {
	return g.Context
}

func NewGuardian(dismissible bool, context string) Guardian {
	return Guardian{
		Dismissible: dismissible,
		Context:     context,
	}
}

func IsDismissible(err error) bool {
	g, ok := err.(Guardian)
	if !ok {
		return false
	}

	return g.Dismissible
}
