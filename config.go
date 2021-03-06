package confMan

import (
	"fmt"
	"path/filepath"
	"reflect"
)

// Provider is used to provide values
// It can implement either Unmarshaler or Filler interface or both
// Name method is used for error messages
type Provider interface {
	Name() string
}

// Unmarshaler can be implemented by providers to receive struct pointer and unmarshal values into it
type Unmarshaler interface {
	UnmarshalStruct(i interface{}) (err error)
}

// Filler can be implemented by providers to receive struct fields and set their value
type Filler interface {
	Fill(in *Input) (err error)
}

// Config loads values from specified providers into given struct
type Config struct {
	// Providers are applied at the order specified
	// If multiple values are provided for a field, last one will get applied
	Providers []Provider

	// Collection of errors during loading values into provided struct
	ce ConfigErrors
}

// Load creates a new Config object
func Load() *Config {
	return &Config{}
}

// FromEnv adds an EnvProvider to Providers list
func (c *Config) FromEnv() *Config {
	return c.AddProvider(NewEnvProvider())
}

// FromFile adds a FileProvider to Providers list
// In case of .env file, it adds a EnvProvider to the list
func (c *Config) FromFile(path string) *Config {
	if filepath.Ext(path) == ENV {
		ep := NewEnvProvider()
		ep.Source = path
		return c.AddProvider(NewEnvProvider())
	}

	return c.AddProvider(NewFileProvider(path))
}

// AddProvider adds a Provider to Providers list
func (c *Config) AddProvider(p Provider) *Config {
	c.Providers = append(c.Providers, p)
	return c
}

// Into will apply all specified providers in order declared
// and validate final struct for required and default fields
// If multiple values are provided for a field, last one will get applied
func (c *Config) Into(i interface{}) error {
	in, err := NewInput(i)
	if err != nil {
		return err
	}

	for _, p := range c.Providers {
		if u, ok := p.(Unmarshaler); ok {
			if err := u.UnmarshalStruct(i); err != nil {
				c.collectError(fmt.Errorf("%v: %w", p.Name(), err))
			}
		}

		if f, ok := p.(Filler); ok {
			if err := f.Fill(in); err != nil {
				c.collectError(fmt.Errorf("%v: %w", p.Name(), err))
			}
		}
	}

	for _, f := range in.Fields {
		if !f.IsSet {
			if f.Tags.Required {
				c.collectError(fmt.Errorf(requiredFieldErrFormat, ErrRequiredField, in.getPath(f.Path)))
			} else if f.Tags.Default != "" {
				err := in.SetValue(f, f.Tags.Default)
				if err != nil {
					c.collectError(err)
				}
			}
		}

		if f.Tags.Expand && f.Value.Kind() == reflect.String {
			err := in.SetValue(f, f.Value.String())
			if err != nil {
				c.collectError(err)
			}
		}
	}

	if len(c.ce) != 0 {
		return c.ce
	}

	return nil
}

func (c *Config) collectError(e error) {
	c.ce = append(c.ce, e)
}
