package notifications

import "golang.org/x/xerrors"

type ProviderRegistry[T Provider] struct {
	providers map[string]T
}

type Provider interface {
	Name() string
}

func NewProviderRegistry[T Provider](providers ...T) (*ProviderRegistry[T], error) {
	reg := &ProviderRegistry[T]{
		providers: make(map[string]T),
	}

	for _, p := range providers {
		if err := reg.Register(p); err != nil {
			return nil, err
		}
	}

	return reg, nil
}

func (p *ProviderRegistry[T]) Register(provider T) error {
	name := provider.Name()
	if _, found := p.providers[name]; found {
		return xerrors.Errorf("%q already registered", name)
	}

	p.providers[name] = provider
	return nil
}

func (p *ProviderRegistry[T]) Resolve(name string) (out T, err error) {
	out, found := p.providers[name]
	if !found {
		// "out" is just used here for the zero-value of the generic type T
		return out, xerrors.Errorf("unknown provider %q", name)
	}

	return out, nil
}
