package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v2"
	"reflect"
	"strconv"
)

func yamlToJSON(y []byte, jsonTarget *reflect.Value, yamlUnmarshal func([]byte, interface{}) error) ([]byte, error) {

	var yamlObj interface{}
	err := yamlUnmarshal(y, &yamlObj)
	if err != nil {
		return nil, err
	}

	// YAML对象与JSON对象不完全兼容（例如，YAML中可以有非字符串的键）
	// 因此，将YAML兼容的对象转换为JSON兼容的对象，如果途中发生不可恢复的 不兼容的情况下，会出现错误
	jsonObj, err := convertToJSONableObject(yamlObj, jsonTarget)
	if err != nil {
		return nil, err
	}

	return json.Marshal(jsonObj)
}

func convertToJSONableObject(yamlObj interface{}, jsonTarget *reflect.Value) (interface{}, error) {
	var err error

	//将jsonTarget解析为一个具体的值（即不是一个指针或一个接口）
	//我们将decodingNull传递为false，因为我们实际上并没有解码到这个值，我们只是在检查最终的目标是否是一个字符串
	if jsonTarget != nil {
		ju, tu, pv := indirect(*jsonTarget, false)
		// 我们在这一层有一个JSON或Text Umarshaler，所以我们不可以解码成一个字符串
		if ju != nil || tu != nil {
			jsonTarget = nil
		} else {
			jsonTarget = &pv
		}
	}

	// 如果yamlObj是一个数字或布尔值，检查jsonTarget是否是一个字符串，如果是，则强制执行
	//否则返回正常
	//如果yamlObj是一个map或数组，找到每个键解析的字段，将该字段的reflect.Value传回这个函数。
	switch typedYAMLObj := yamlObj.(type) {
	case map[interface{}]interface{}:
		// JSON不支持map中的任意key，所以我们必须将这些键转换成字符串 ，键只能有字符串、int、int64、float64、binary (不支持），或null（不支持）
		strMap := make(map[string]interface{})
		for k, v := range typedYAMLObj {
			// 首先将key解析成一个字符串
			var keyString string
			switch typedKey := k.(type) {
			case string:
				keyString = typedKey
			case int:
				keyString = strconv.Itoa(typedKey)
			case int64:
				keyString = strconv.FormatInt(typedKey, 10)
			case float64:
				s := strconv.FormatFloat(typedKey, 'g', -1, 32)
				switch s {
				case "+Inf":
					s = ".inf"
				case "-Inf":
					s = "-.inf"
				case "NaN":
					s = ".nan"
				}
				keyString = s
			case bool:
				if typedKey {
					keyString = "true"
				} else {
					keyString = "false"
				}
			default:
				return nil, fmt.Errorf("Unsupported map key of type: %s, key: %+#v, value: %+#v",
					reflect.TypeOf(k), k, v)
			}

			// jsonTarget应该是一个结构体或一个map
			// 如果是结构体，找到它要映射到的字段，并传递它的reflect.Value
			// 如果是map，找到map的元素类型并传递从该类型创建的reflect.Value
			// 如果它既不是，就传给 nil
			if jsonTarget != nil {
				t := *jsonTarget
				if t.Kind() == reflect.Struct {
					keyBytes := []byte(keyString)

					var f *field
					fields := cachedTypeFields(t.Type())
					for i := range fields {
						ff := &fields[i]
						if bytes.Equal(ff.nameBytes, keyBytes) {
							f = ff
							break
						}

						if f == nil && ff.equalFold(ff.nameBytes, keyBytes) {
							f = ff
						}
					}
					if f != nil {

						jtf := t.Field(f.index[0])
						strMap[keyString], err = convertToJSONableObject(v, &jtf)
						if err != nil {
							return nil, err
						}
						continue
					}
				} else if t.Kind() == reflect.Map {

					jtv := reflect.Zero(t.Type().Elem())
					strMap[keyString], err = convertToJSONableObject(v, &jtv)
					if err != nil {
						return nil, err
					}
					continue
				}
			}
			strMap[keyString], err = convertToJSONableObject(v, nil)
			if err != nil {
				return nil, err
			}
		}
		return strMap, nil
	case []interface{}:
		// 我们需要对数组进行递归，以防里面有map[interface{}]interface{}，并将任何数字转换成字符串
		// 如果jsonTarget是一个slice，找到它要映射的东西，如果它不是一个slice，就传递nil
		var jsonSliceElemValue *reflect.Value
		if jsonTarget != nil {
			t := *jsonTarget
			if t.Kind() == reflect.Slice {
				// 默认情况下，切片指向nil，但是我们需要一个reflect.Value，指向一个切片类型的值，所以我们在这里创建一个。
				ev := reflect.Indirect(reflect.New(t.Type().Elem()))
				jsonSliceElemValue = &ev
			}
		}

		arr := make([]interface{}, len(typedYAMLObj))
		for i, v := range typedYAMLObj {
			arr[i], err = convertToJSONableObject(v, jsonSliceElemValue)
			if err != nil {
				return nil, err
			}
		}
		return arr, nil
	default:
		// 如果目标类型是一个字符串，而YAML类型是一个数字，将YAML类型转换为字符串
		if jsonTarget != nil && (*jsonTarget).Kind() == reflect.String {
			var s string
			switch typedVal := typedYAMLObj.(type) {
			case int:
				s = strconv.FormatInt(int64(typedVal), 10)
			case int64:
				s = strconv.FormatInt(typedVal, 10)
			case float64:
				s = strconv.FormatFloat(typedVal, 'g', -1, 32)
			case uint64:
				s = strconv.FormatUint(typedVal, 10)
			case bool:
				if typedVal {
					s = "true"
				} else {
					s = "false"
				}
			}
			if len(s) > 0 {
				yamlObj = interface{}(s)
			}
		}
		return yamlObj, nil
	}

	return nil, nil
}

// JSON to YAML.
func JSONToYAML(j []byte) ([]byte, error) {
	var jsonObj interface{}

	err := yaml.Unmarshal(j, &jsonObj)
	if err != nil {
		return nil, err
	}

	return yaml.Marshal(jsonObj)
}

// YAMLToJSON
func YAMLToJSON(y []byte) ([]byte, error) {
	return yamlToJSON(y, nil, yaml.Unmarshal)
}

func Marshal(o interface{}) ([]byte, error) {
	j, err := json.Marshal(o)
	if err != nil {
		return nil, fmt.Errorf("error marshaling into JSON: %v", err)
	}

	y, err := JSONToYAML(j)
	if err != nil {
		return nil, fmt.Errorf("error converting JSON to YAML: %v", err)
	}

	return y, nil
}
