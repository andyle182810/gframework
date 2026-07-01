package transformer_test

import (
	"context"
	"testing"

	"github.com/andyle182810/gframework/transformer"
	"github.com/go-playground/mold/v4"
	"github.com/stretchr/testify/require"
)

const (
	loweredEmail = "joe@example.com"
	bloggsName   = "Joe Bloggs"
)

type modifierTarget struct {
	Name    string `mod:"trim"`
	Email   string `mod:"trim,lcase"`
	Role    string `mod:"default=member"`
	Comment string // no mod tag — must be left untouched
}

func TestTransform_AppliesModifiers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   modifierTarget
		want modifierTarget
	}{
		{
			name: "trims and lowercases",
			in:   modifierTarget{Name: "  Joe  ", Email: "  Joe@Example.COM  ", Role: "", Comment: "  keep  "},
			want: modifierTarget{Name: "Joe", Email: loweredEmail, Role: "member", Comment: "  keep  "},
		},
		{
			name: "applies default to empty field only",
			in:   modifierTarget{Name: "Ann", Email: "ann@x.io", Role: "admin", Comment: "x"},
			want: modifierTarget{Name: "Ann", Email: "ann@x.io", Role: "admin", Comment: "x"},
		},
	}

	tfm := transformer.DefaultRestTransformer()

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := testCase.in
			require.NoError(t, tfm.Transform(context.Background(), &got))
			require.Equal(t, testCase.want, got)
		})
	}
}

type scrubTarget struct {
	Name  string `scrub:"name"`
	Email string `scrub:"emails"`
}

func TestScrub_RedactsPII(t *testing.T) {
	t.Parallel()

	tfm := transformer.DefaultRestTransformer()
	target := scrubTarget{Name: bloggsName, Email: loweredEmail}

	require.NoError(t, tfm.Scrub(context.Background(), &target))

	require.Contains(t, target.Name, "<<scrubbed::name::sha1::")
	require.NotContains(t, target.Name, bloggsName)

	require.Contains(t, target.Email, "<<scrubbed::email::sha1::")
	require.NotContains(t, target.Email, loweredEmail)
}

func TestScrubbedCopy_DoesNotMutateOriginal(t *testing.T) {
	t.Parallel()

	tfm := transformer.DefaultRestTransformer()
	original := &scrubTarget{Name: bloggsName, Email: loweredEmail}

	scrubbed, err := transformer.ScrubbedCopy(context.Background(), tfm, original)
	require.NoError(t, err)

	// Original is untouched.
	require.Equal(t, bloggsName, original.Name)
	require.Equal(t, loweredEmail, original.Email)

	// Copy is redacted.
	require.Contains(t, scrubbed.Name, "<<scrubbed::name::sha1::")
	require.NotContains(t, scrubbed.Email, loweredEmail)
}

func TestScrubbedCopyValue(t *testing.T) {
	t.Parallel()

	tfm := transformer.DefaultRestTransformer()
	original := &scrubTarget{Name: bloggsName, Email: loweredEmail}

	out, err := transformer.ScrubbedCopyValue(context.Background(), tfm, original)
	require.NoError(t, err)

	scrubbed, ok := out.(*scrubTarget)
	require.True(t, ok)

	// Copy is redacted.
	require.Contains(t, scrubbed.Name, "<<scrubbed::name::sha1::")
	require.NotContains(t, scrubbed.Email, loweredEmail)

	// Original is untouched.
	require.Equal(t, bloggsName, original.Name)
	require.Equal(t, loweredEmail, original.Email)

	// Non-transformable inputs yield ErrNotTransformable and no value.
	notPtr, err := transformer.ScrubbedCopyValue(context.Background(), tfm, scrubTarget{Name: "x", Email: "y"})
	require.ErrorIs(t, err, transformer.ErrNotTransformable)
	require.Nil(t, notPtr)
}

func TestRegisterModifier_Custom(t *testing.T) {
	t.Parallel()

	tfm := transformer.DefaultRestTransformer()
	tfm.RegisterModifier("reverse", func(_ context.Context, fld mold.FieldLevel) error {
		runes := []rune(fld.Field().String())

		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}

		fld.Field().SetString(string(runes))

		return nil
	})

	target := struct {
		Code string `mod:"reverse"`
	}{Code: "abc"}

	require.NoError(t, tfm.Transform(context.Background(), &target))
	require.Equal(t, "cba", target.Code)
}

func TestIsTransformable(t *testing.T) {
	t.Parallel()

	type sample struct{ A string }

	var nilPtr *sample

	tests := []struct {
		name string
		in   any
		want bool
	}{
		{name: "pointer to struct", in: &sample{A: ""}, want: true},
		{name: "non-pointer struct", in: sample{A: ""}, want: false},
		{name: "nil pointer", in: nilPtr, want: false},
		{name: "pointer to slice", in: &[]string{}, want: false},
		{name: "primitive", in: "x", want: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, testCase.want, transformer.IsTransformable(testCase.in))
		})
	}
}
