package lua

import (
	"errors"
	"fmt"
	"unsafe"
)

var lua_tointegerx = luaDLL.NewProc("lua_tointegerx")

func (this Lua) ToInteger(index int) (int, error) {
	var issucceeded uintptr
	value, _, _ := lua_tointegerx.Call(this.State(), uintptr(index),
		uintptr(unsafe.Pointer(&issucceeded)))
	if issucceeded != 0 {
		return int(value), nil
	} else {
		return 0, errors.New("ToInteger: the value in not integer on the stack")
	}
}

var lua_tolstring = luaDLL.NewProc("lua_tolstring")

func (this Lua) ToAnsiString(index int) []byte {
	var length uintptr
	p, _, _ := lua_tolstring.Call(this.State(),
		uintptr(index),
		uintptr(unsafe.Pointer(&length)))
	if length <= 0 {
		return []byte{}
	} else {
		return CGoBytes(p, length)
	}
}

func (this Lua) ToString(index int) (string, error) {
	var length uintptr
	p, _, _ := lua_tolstring.Call(this.State(),
		uintptr(index),
		uintptr(unsafe.Pointer(&length)))
	return CGoStringN(p, length), nil
}

var lua_touserdata = luaDLL.NewProc("lua_touserdata")

func (this Lua) ToUserData(index int) unsafe.Pointer {
	rv, _, _ := lua_touserdata.Call(this.State(), uintptr(index))
	return unsafe.Pointer(rv)
}

var lua_toboolean = luaDLL.NewProc("lua_toboolean")

func (this Lua) ToBool(index int) bool {
	rv, _, _ := lua_toboolean.Call(this.State(), uintptr(index))
	return rv != 0
}

type TString struct {
	Value []byte
}

func (this *TString) String() (string, error) {
	if len(this.Value) <= 0 {
		return "", nil
	} else {
		return string(this.Value), nil
	}
}

func (this *TString) Push(L Lua) int {
	L.PushAnsiString(this.Value)
	return 1
}

var lua_tocfunction = luaDLL.NewProc("lua_tocfunction")

func (this *Lua) ToCFunction(index int) uintptr {
	rc, _, _ := lua_tocfunction.Call(this.State(), uintptr(index))
	return rc
}

type TFunction struct {
	IsCFunc bool
	Address uintptr
	Chank   []byte
}

func (this TFunction) Push(L Lua) int {
	if this.IsCFunc {
		// CFunction
		L.PushCFunction(this.Address)
	} else {
		// LuaFunction
		err := L.LoadBufferX("(anonymous)", this.Chank, "b")
		if err != nil {
			return 0
		}
	}
	return 1
}

type TLightUserData struct {
	Data unsafe.Pointer
}

func (this TLightUserData) Push(L Lua) int {
	L.PushLightUserData(this.Data)
	return 1
}

type TFullUserData []byte

func (this TFullUserData) Push(L Lua) int {
	size := len([]byte(this))
	p := L.NewUserData(uintptr(size))
	for i := 0; i < size; i++ {
		*(*byte)(unsafe.Pointer(uintptr(p) + uintptr(i))) = this[i]
	}
	return 1
}

var lua_next = luaDLL.NewProc("lua_next")

func (this Lua) Next(index int) int {
	rc, _, _ := lua_next.Call(this.State(), uintptr(index))
	return int(rc)
}

var lua_rawlen = luaDLL.NewProc("lua_rawlen")

func (this Lua) RawLen(index int) uintptr {
	size, _, _ := lua_rawlen.Call(this.State(), uintptr(index))
	return size
}

func (this Lua) ToSomething(index int) (interface{}, error) {
	switch this.GetType(index) {
	case LUA_TBOOLEAN:
		return this.ToBool(index), nil
	case LUA_TFUNCTION:
		if p := this.ToCFunction(index); p != 0 {
			// CFunction
			return TFunction{IsCFunc: true, Address: p}, nil
		} else {
			// LuaFunction
			return TFunction{IsCFunc: false, Chank: this.Dump()}, nil
		}
	case LUA_TLIGHTUSERDATA:
		return &TLightUserData{this.ToUserData(index)}, nil
	case LUA_TNIL:
		return nil, nil
	case LUA_TNUMBER:
		return this.ToInteger(index)
	case LUA_TSTRING:
		return TString{this.ToAnsiString(index)}, nil
	case LUA_TTABLE:
		top := this.GetTop()
		defer this.SetTop(top)
		table := map[string]interface{}{}
		this.PushNil()
		if index < 0 {
			index--
		}
		for this.Next(index) != 0 {
			key, keyErr := this.ToSomething(-2)
			if keyErr == nil {
				val, valErr := this.ToSomething(-1)
				if valErr == nil {
					switch t := key.(type) {
					case string:
						table[t] = val
					case int:
						table[fmt.Sprintf("%d", t)] = val
					case nil:
						table[""] = val
					}
				}
			}
			this.Pop(1)
		}
		return table, nil
	case LUA_TUSERDATA:
		size := this.RawLen(index)
		ptr := this.ToUserData(index)
		return TFullUserData(CGoBytes(uintptr(ptr), uintptr(size))), nil
	default:
		return nil, errors.New("lua.ToSomeThing: Not supported type found.")
	}
}
