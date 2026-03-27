package debugtools

import (
	"testing"
)

func TestSetting_Name(t *testing.T) {
	tests := []struct {
		name    string
		setting *Setting
		want    string
	}{
		{
			name:    "Get setting name",
			setting: NewSetting("testsetting"),
			want:    "testsetting",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.setting.Name(); got != tt.want {
				t.Errorf("Setting.Name() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetting_Value(t *testing.T) {
	tests := []struct {
		name       string
		newSetting func() *Setting
		want       string
	}{
		{
			name: "Read setting value",
			newSetting: func() *Setting {
				const featureName = "testfeature2"
				const env = "testfeature1=1," + featureName + "=somevalue,testfeature3=3"

				parse(env)
				return NewSetting(featureName)
			},
			want: "somevalue",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.newSetting()
			if got := s.Value(); got != tt.want {
				t.Errorf("Setting.Value() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetting_String(t *testing.T) {
	tests := []struct {
		name       string
		newSetting func() *Setting
		want       string
	}{
		{
			name: "Setting kv",
			newSetting: func() *Setting {
				const featureName = "testfeature2"
				const env = "testfeature1=1," + featureName + "=somevalue,testfeature3=3"

				parse(env)
				return NewSetting(featureName)
			},
			want: "testfeature2=somevalue",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.newSetting()
			if got := s.String(); got != tt.want {
				t.Errorf("Setting.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
