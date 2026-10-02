package v8go_test

import (
	"fmt"

	v8 "github.com/ad3n/v8go"
)

func SetObjectTemplateAccessorProperty(
	iso *v8.Isolate,
	templ *v8.ObjectTemplate,
	key string,
	get v8.FunctionCallbackWithError,
	set v8.FunctionCallbackWithError,
	attributes v8.PropertyAttribute,
) {
	var (
		v8get *v8.FunctionTemplate
		v8set *v8.FunctionTemplate
	)
	if get != nil {
		v8get = v8.NewFunctionTemplateWithError(iso, get)
	}

	if set != nil {
		v8set = v8.NewFunctionTemplateWithError(iso, set)
	}

	templ.SetAccessorProperty(key, v8get, v8set, attributes)
}

func ExampleObjectTemplate_SetAccessorProperty_helpers() {
	iso := v8.NewIsolate()
	defer iso.Dispose()
	tmpl := v8.NewObjectTemplate(iso)

	current, _ := v8.NewValue(iso, "current")
	SetObjectTemplateAccessorProperty(iso, tmpl,
		"prop",

		func(*v8.FunctionCallbackInfo) (*v8.Value, error) {
			return current, nil
		},

		func(info *v8.FunctionCallbackInfo) (*v8.Value, error) {
			current = info.Args()[0]
			return nil, nil
		},
		v8.None,
	)

	global := v8.NewObjectTemplate(iso)
	global.Set("obj", tmpl)
	ctx := v8.NewContext(iso, global)
	defer ctx.Close()

	value, _ := ctx.RunScript("obj.prop", "")
	fmt.Printf("Property value before set: %s\n", value.String())

	value, _ = ctx.RunScript("obj.prop = 'new value'; obj.prop", "")
	fmt.Printf("Property value after set: %s\n", value.String())

}
