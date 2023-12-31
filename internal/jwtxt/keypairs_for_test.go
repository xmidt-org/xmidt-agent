// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package jwtxt

const (
	pemECPublic = "" +
		"-----BEGIN PUBLIC KEY-----\n" +
		"MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE2JebmtU5WHi5yHBHmzhyiEGbg6OL\n" +
		"r463xYdqs/Nzlh2OkaIikanpi7opOuD6wiqFVd9xaMjA54L5vjb5oLcLuA==\n" +
		"-----END PUBLIC KEY-----"

	pemECPrivate = "" +
		"-----BEGIN EC PRIVATE KEY-----\n" +
		"MHcCAQEEIHJCsQFvPLEV45BXU3DLWEVUPiKSYte8knw7ZtrIj6YxoAoGCCqGSM49\n" +
		"AwEHoUQDQgAE2JebmtU5WHi5yHBHmzhyiEGbg6OLr463xYdqs/Nzlh2OkaIikanp\n" +
		"i7opOuD6wiqFVd9xaMjA54L5vjb5oLcLuA==\n" +
		"-----END EC PRIVATE KEY-----"

	pemEdPublic = "" +
		"-----BEGIN PUBLIC KEY-----\n" +
		"MCowBQYDK2VwAyEA0WQIwE/DiCikp79XIkJ0H1vDiERaOieGL/1N8B+k7s8=\n" +
		"-----END PUBLIC KEY-----\n"

	pemEdPrivate = "" +
		"-----BEGIN PRIVATE KEY-----\n" +
		"MC4CAQAwBQYDK2VwBCIEIHdPSdNde11yNaBYj+q/4044LbOo2lVAb73u7aL13UcH\n" +
		"-----END PRIVATE KEY-----"

	pemRSAPublic = "" +
		"-----BEGIN PUBLIC KEY-----\n" +
		"MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAx3HGBMr6UCtsABqMkG9s\n" +
		"w0DLRuRZK9M4b535T4vC3i37+3YCLHB9wvOhEOo6b7h6lJehX9Px7pL3ppWu+tr9\n" +
		"LuCxW+Nz46gpgKAXvVbbuc7VU2O0XUBuus0WsOgUQUzqHN6ZNpA/eY3mMndEKR79\n" +
		"DWdJMSNylBPGvS54WEtgIE8hDor/pPx/cTleXGXq3DasfqnoOlD/ALKL0eqkzbnX\n" +
		"GGUzN2K79RCw7mm/CQeS5a7mLgRypT83fR3Kg1SgsyXCUjTNPupQVgWggxfWbRIj\n" +
		"T5Q1LkBRl3SDKM6OaPb3xh5NncuQktbjSFO5NLlGdL6Ylzfm0OlK3nBvrpfmac46\n" +
		"7QIDAQAB\n" +
		"-----END PUBLIC KEY-----\n"

	pemRSAPrivate = "" +
		"-----BEGIN RSA PRIVATE KEY-----\n" +
		"MIIEowIBAAKCAQEAx3HGBMr6UCtsABqMkG9sw0DLRuRZK9M4b535T4vC3i37+3YC\n" +
		"LHB9wvOhEOo6b7h6lJehX9Px7pL3ppWu+tr9LuCxW+Nz46gpgKAXvVbbuc7VU2O0\n" +
		"XUBuus0WsOgUQUzqHN6ZNpA/eY3mMndEKR79DWdJMSNylBPGvS54WEtgIE8hDor/\n" +
		"pPx/cTleXGXq3DasfqnoOlD/ALKL0eqkzbnXGGUzN2K79RCw7mm/CQeS5a7mLgRy\n" +
		"pT83fR3Kg1SgsyXCUjTNPupQVgWggxfWbRIjT5Q1LkBRl3SDKM6OaPb3xh5NncuQ\n" +
		"ktbjSFO5NLlGdL6Ylzfm0OlK3nBvrpfmac467QIDAQABAoIBADvDuaFdC6YzZNEh\n" +
		"I4byhMZ7p45ORfROfoZf8cHm8RVv9SbUpXEYom7lX5n4fltVDhJx348eLUye6KwY\n" +
		"BY+xSJYgCbWt0l/hV9Jt5r87hGtI8f7jjTw2XxgF9es8GDm7KRpOj93cWtD7dwQf\n" +
		"XiLuYMj/7txVMXPy+yZcgv5+U8dKN18EUbUWxxH+JvS/BHA2klhVMY/S6wneiGli\n" +
		"i5ZWFag/NAWWFH5rY0pYZIJ059xzzaDQGXihuAq6MhhoRhvwh50HxUKGnDHNGJ7Q\n" +
		"MFs7Mbr16gYDgOGlHT2HGDsdvKp9X3KVyQmUNtCNX2B9CRX1b1d3ofey9VD/tMAT\n" +
		"07GE/pcCgYEA7ILWxRFR1nTHc+nNdi39nwULkwsO1Qwc3XKnXPqxl/F2M4HZFGHR\n" +
		"rcaWBZ/sTqj8P52AgJq9QNoOZ9dKCpCVfPawHYo6zyPb0XF9Od6mT3KAm8AeDLya\n" +
		"0yrh0XCnOhzS09dTueNXbUYIDlHFkK8WXF+J0Gwh0oxEtSnZUf2P+F8CgYEA1+EF\n" +
		"CKAiqfd+vKyku6FjoE89O1dc4CuJEMXGhgZ88rn4fec3Eqms6155+4DMwwyo0hvF\n" +
		"nyoBeb/5/WJJm2EKnbSjSJ9uxFSeguIeIC2SiZCnQrEwUPMzcG9UjOr9UB67cim7\n" +
		"N9d+kcV4DFe2knBqv7Iuvk0YLF6X7XBAXzT4QDMCgYEA43iTh8Y4p8J5coqUCe4B\n" +
		"2EfJ8grYoR+dQ39aaJrU5AZgYPmqB2htem1dLNu7M4xjz+t0BDzPeOhAoq71j2Ov\n" +
		"4xiAGmkwVrluWeqFPnteCVtfRm1oeWeMoTzFI+Ltc371Zrna1RZKp9aLOPp8wcMk\n" +
		"BoP80HCvtwkhq/wsACeXqJECgYAdSJ7gLqjFGZeNjHXEJf5XrqgFtrIYjo9HQSzO\n" +
		"3W5xlpyIp6am13FndCdj4HLmOn9kEPRbxNzyYQJORtjpRN6lye0kWswxwbDG3Flt\n" +
		"0ADCvGaT+2kscfEWXWPAwdee2KxgrhyBVLAMohbIxdU0RB+W5VrF4btXuXUudj2l\n" +
		"LJBIVQKBgCHdEUCWOacO5+B/yZijAbRTnKHH4Ht3B3bYJy6M5qIysSVzG4/Pls90\n" +
		"bAQvlAyBG5I+yY1zBfeziI6Uopg/XCrxNNv+8sxiPXt1D8QkCPN08wKulj7DnCuf\n" +
		"re3wR8zUZNUhRARSWJa68r7sDJVxxDxZjeh3OywmEBPYpQTBM5sa\n" +
		"-----END RSA PRIVATE KEY-----\n"
)

func publicECOption() Option {
	return WithPEMs([]byte(pemECPublic))
}

func anyPublicOption() Option {
	return WithPEMs([]byte(pemECPublic), []byte(pemEdPublic), []byte(pemRSAPublic))
}
