package goluaole

import (
	"errors"

	lua "../../lua"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

var initialized_required = true

type capsule_t struct {
	Data *ole.IDispatch
}

type method_t struct {
	Name string
}

const OBJECT_T = "OLE_OBJECT"
const METHOD_T = "OLE_METHOD"

func (this capsule_t) Push(L lua.Lua) int {
	L.PushUserData(&this)
	L.NewMetaTable(OBJECT_T)
	L.PushGoFunction(gc)
	L.SetField(-2, "__gc")
	L.PushGoFunction(index)
	L.SetField(-2, "__index")
	L.SetMetaTable(-2)
	return 1
}

func gc(L lua.Lua) int {
	p := capsule_t{}
	sync := L.ToUserDataTo(1, &p)
	defer sync()
	if p.Data != nil {
		p.Data.Release()
		p.Data = nil
	}
	return 0
}

func lua2interface(L lua.Lua, index int) (interface{}, error) {
	switch L.GetType(index) {
	default:
		return nil, nil
	case lua.LUA_TSTRING:
		str, str_err := L.ToString(index)
		return str, str_err
	case lua.LUA_TNUMBER:
		num, num_err := L.ToInteger(index)
		return num, num_err
	case lua.LUA_TUSERDATA:
		data, data_err := L.TestUData(index, OBJECT_T)
		if data_err != nil {
			return nil, data_err
		}
		if data == nil {
			return nil, errors.New("Not A OLE-Object")
		}
		val := (*capsule_t)(data)
		return val.Data, nil
	case lua.LUA_TBOOLEAN:
		return L.ToBool(index), nil
	}
}

func lua2interfaceS(L lua.Lua, start, end int) ([]interface{}, error) {
	result := make([]interface{}, 0, end-start+1)
	for i := start; i <= end; i++ {
		val, val_err := lua2interface(L, i)
		if val_err != nil {
			return nil, val_err
		}
		result = append(result, val)
	}
	return result, nil
}

// this:_call("METHODNAME",params...)
func call1(L lua.Lua) int {
	p, p_err := (L.TestUData(1, OBJECT_T))
	if p_err != nil {
		return L.Push(nil, p_err)
	}
	if p == nil {
		return L.Push(nil, "OLEOBJECT._call: the receiver is null")
	}
	name, name_err := L.ToString(2)
	if name_err != nil {
		return L.Push(nil, name_err)
	}
	return call_common(L, ((*capsule_t)(p)).Data, name)
}

// this:METHODNAME(params...)
func call2(L lua.Lua) int {
	m, m_err := L.TestUData(1, METHOD_T)
	if m_err != nil {
		return L.Push(nil, m_err)
	}
	if m == nil || ((*method_t)(m)).Name == "" {
		return L.Push(nil, "OLEOBJECT(): the method is null")
	}
	p, p_err := L.TestUData(2, OBJECT_T)
	if p_err != nil {
		return L.Push(nil, p_err)
	}
	if p == nil || ((*capsule_t)(p)).Data == nil {
		return L.Push(nil, "OLEOBJECT(): the receiver is null")
	}
	return call_common(L, ((*capsule_t)(p)).Data, ((*method_t)(m)).Name)
}

func call_common(L lua.Lua, com1 *ole.IDispatch, name string) int {
	count := L.GetTop()
	params, params_err := lua2interfaceS(L, 3, count)
	if params_err != nil {
		return L.Push(nil, params_err)
	}
	result, result_err := oleutil.CallMethod(com1, name, params...)
	if result_err != nil {
		return L.Push(nil, result_err)
	}
	if result.VT == ole.VT_DISPATCH {
		return capsule_t{result.ToIDispatch()}.Push(L)
	} else {
		return L.Push(result.Value())
	}
}

func set(L lua.Lua) int {
	p, p_err := (L.TestUData(1, OBJECT_T))
	if p_err != nil {
		return L.Push(nil, p_err)
	}
	if p == nil {
		return L.Push(nil, "OLEOBJECT._set: the receiver is null")
	}
	name, name_err := L.ToString(2)
	if name_err != nil {
		return L.Push(nil, name_err)
	}
	key, key_err := lua2interfaceS(L, 3, L.GetTop())
	if key_err != nil {
		return L.Push(nil, key_err)
	}
	oleutil.PutProperty(((*capsule_t)(p)).Data, name, key...)
	return L.Push(true, nil)
}

func get(L lua.Lua) int {
	p, p_err := L.TestUData(1, OBJECT_T)
	if p_err != nil {
		return L.Push(nil, p_err)
	}
	if p == nil {
		return L.Push(nil, "OLEOBJECT._set: the receiver is null")
	}
	name, name_err := L.ToString(2)
	if name_err != nil {
		return L.Push(nil, name_err)
	}
	key, key_err := lua2interfaceS(L, 3, L.GetTop())
	if key_err != nil {
		return L.Push(nil, key_err)
	}
	result, result_err := oleutil.GetProperty(((*capsule_t)(p)).Data, name, key...)
	if result_err != nil {
		return L.Push(nil, result_err)
	}
	if result.VT == ole.VT_DISPATCH {
		return capsule_t{result.ToIDispatch()}.Push(L)
	} else {
		return L.Push(result.Value())
	}
}

func index(L lua.Lua) int {
	name, name_err := L.ToString(2)
	if name_err != nil {
		return L.Push(nil, name_err)
	}
	switch name {
	case "_call":
		return L.Push(call1, nil)
	case "_set":
		return L.Push(set, nil)
	case "_get":
		return L.Push(get, nil)
	default:
		L.PushUserData(&method_t{Name: name})
		L.NewMetaTable(METHOD_T)
		L.PushGoFunction(call2)
		L.SetField(-2, "__call")
		L.SetMetaTable(-2)
		return 1
	}
}

func CreateObject(L lua.Lua) int {
	if initialized_required {
		ole.CoInitialize(0)
		initialized_required = false
	}
	name, name_err := L.ToString(1)
	if name_err != nil {
		return L.Push(nil, name_err)
	}
	unknown, unknown_err := oleutil.CreateObject(name)
	if unknown_err != nil {
		return L.Push(nil, unknown_err)
	}
	obj, obj_err := unknown.QueryInterface(ole.IID_IDispatch)
	if obj_err != nil {
		return L.Push(nil, obj_err)
	}
	capsule_t{obj}.Push(L)
	return 1
}
