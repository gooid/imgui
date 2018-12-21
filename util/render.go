// Copyright 2017 The gooid Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"fmt"
	"log"
	"unsafe"

	gl "github.com/gooid/gl/es2"
	"github.com/gooid/imgui"
)

type Render struct {
	glslVersion            string
	fontTexture            uint32
	shaderHandle           uint32
	vertHandle             uint32
	fragHandle             uint32
	attribLocationTex      int32
	attribLocationProjMtx  int32
	attribLocationPosition int32
	attribLocationUV       int32
	attribLocationColor    int32
	//vboHandle              uint32
	elementsHandle uint32
}

func NewRender(glslVersion string) *Render {
	if glslVersion == "" {
		glslVersion = "#version 300 es"
	}
	return &Render{
		glslVersion: glslVersion,
	}
}

// indexSize := imgui.IndexBufferLayout()
//vertexSize, vertexOffsetPos, vertexOffsetUv, vertexOffsetCol := imgui.VertexBufferLayout()
func (impl *Render) Render(drawData imgui.DrawData) {
	// Avoid rendering when minimized, scale coordinates for retina displays (screen coordinates != framebuffer coordinates)
	scale := imgui.GetIO().GetDisplayFramebufferScale()
	displaySize := drawData.GetDisplaySize()
	fbWidth := int(displaySize.GetX() * scale.GetX())
	fbHeight := int(displaySize.GetY() * scale.GetY())
	if fbWidth <= 0 || fbHeight <= 0 {
		return
	}
	drawData.ScaleClipRects(scale)
	indexSize := imgui.GetDrawIdxSize()
	vertexSize := imgui.GetDrawVertSize()
	var vertexOffsetPos, vertexOffsetUv, vertexOffsetCol int
	imgui.DrawVertOffset(&vertexOffsetPos, &vertexOffsetUv, &vertexOffsetCol)

	// Backup GL state
	var lastActiveTexture int32
	gl.GetIntegerv(gl.ACTIVE_TEXTURE, &lastActiveTexture)
	gl.ActiveTexture(gl.TEXTURE0)
	var lastProgram int32
	gl.GetIntegerv(gl.CURRENT_PROGRAM, &lastProgram)
	var lastTexture int32
	gl.GetIntegerv(gl.TEXTURE_BINDING_2D, &lastTexture)
	var lastArrayBuffer int32
	gl.GetIntegerv(gl.ARRAY_BUFFER_BINDING, &lastArrayBuffer)
	//var lastVertexArray int32
	//gl.GetIntegerv(gl.VERTEX_ARRAY_BINDING, &lastVertexArray)
	var lastViewport [4]int32
	gl.GetIntegerv(gl.VIEWPORT, &lastViewport[0])
	var lastScissorBox [4]int32
	gl.GetIntegerv(gl.SCISSOR_BOX, &lastScissorBox[0])
	var lastBlendSrcRgb int32
	gl.GetIntegerv(gl.BLEND_SRC_RGB, &lastBlendSrcRgb)
	var lastBlendDstRgb int32
	gl.GetIntegerv(gl.BLEND_DST_RGB, &lastBlendDstRgb)
	var lastBlendSrcAlpha int32
	gl.GetIntegerv(gl.BLEND_SRC_ALPHA, &lastBlendSrcAlpha)
	var lastBlendDstAlpha int32
	gl.GetIntegerv(gl.BLEND_DST_ALPHA, &lastBlendDstAlpha)
	var lastBlendEquationRgb int32
	gl.GetIntegerv(gl.BLEND_EQUATION_RGB, &lastBlendEquationRgb)
	var lastBlendEquationAlpha int32
	gl.GetIntegerv(gl.BLEND_EQUATION_ALPHA, &lastBlendEquationAlpha)
	lastEnableBlend := gl.IsEnabled(gl.BLEND)
	lastEnableCullFace := gl.IsEnabled(gl.CULL_FACE)
	lastEnableDepthTest := gl.IsEnabled(gl.DEPTH_TEST)
	lastEnableScissorTest := gl.IsEnabled(gl.SCISSOR_TEST)

	// Setup render state: alpha-blending enabled, no face culling, no depth testing, scissor enabled, polygon fill
	gl.Enable(gl.BLEND)
	gl.BlendEquation(gl.FUNC_ADD)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.Disable(gl.CULL_FACE)
	gl.Disable(gl.DEPTH_TEST)
	gl.Enable(gl.SCISSOR_TEST)

	// Setup viewport, orthographic projection matrix
	// Our visible imgui space lies from draw_data->DisplayPps (top left) to draw_data->DisplayPos+data_data->DisplaySize (bottom right). DisplayMin is typically (0,0) for single viewport apps.
	gl.Viewport(0, 0, int32(fbWidth), int32(fbHeight))
	displayPos := drawData.GetDisplayPos()
	L := displayPos.GetX()
	R := displayPos.GetX() + displaySize.GetX()
	T := displayPos.GetY()
	B := displayPos.GetY() + displaySize.GetY()
	orthoProjection := [4][4]float32{
		{2.0 / (R - L), 0.0, 0.0, 0.0},
		{0.0, 2.0 / (T - B), 0.0, 0.0},
		{0.0, 0.0, -1.0, 0.0},
		{(R + L) / (L - R), (T + B) / (B - T), 0.0, 1.0},
	}

	gl.UseProgram(impl.shaderHandle)
	gl.Uniform1i(impl.attribLocationTex, 0)
	gl.UniformMatrix4fv(impl.attribLocationProjMtx, 1, false, &orthoProjection[0][0])

	// Recreate the VAO every time
	// (This is to easily allow multiple GL contexts. VAO are not shared among GL contexts, and we don't track creation/deletion of windows so we don't have an obvious key to use to cache them.)
	//var vaoHandle uint32
	//gl.GenVertexArrays(1, &vaoHandle)
	//gl.BindVertexArray(vaoHandle)
	//gl.BindBuffer(gl.ARRAY_BUFFER, impl.vboHandle)
	gl.EnableVertexAttribArray(uint32(impl.attribLocationPosition))
	gl.EnableVertexAttribArray(uint32(impl.attribLocationUV))
	gl.EnableVertexAttribArray(uint32(impl.attribLocationColor))

	//gl.VertexAttribPointer(uint32(impl.attribLocationPosition), 2, gl.FLOAT, false, int32(vertexSize), unsafe.Pointer(uintptr(vertexOffsetPos)))
	//gl.VertexAttribPointer(uint32(impl.attribLocationUV), 2, gl.FLOAT, false, int32(vertexSize), unsafe.Pointer(uintptr(vertexOffsetUv)))
	//gl.VertexAttribPointer(uint32(impl.attribLocationColor), 4, gl.UNSIGNED_BYTE, true, int32(vertexSize), unsafe.Pointer(uintptr(vertexOffsetCol)))

	drawType := gl.UNSIGNED_SHORT
	if indexSize == 4 {
		drawType = gl.UNSIGNED_INT
	}

	// Draw
	for _, list := range drawData.GetCmdLists() {
		indexBufferOffset := uint(0)

		vertexBuffer := list.VtxBufferAt(0)
		//vertexBufferSize := list.VtxBufferSize()
		indexBuffer, indexBufferSize := list.IdxBufferAt(0), list.IdxBufferSize()

		//gl.BindBuffer(gl.ARRAY_BUFFER, impl.vboHandle)
		//gl.BufferData(gl.ARRAY_BUFFER, int(vertexBufferSize), unsafe.Pointer(vertexBuffer.Swigcptr()), gl.STREAM_DRAW)

		gl.VertexAttribPointer(uint32(impl.attribLocationPosition), 2, gl.FLOAT, false, int32(vertexSize), unsafe.Pointer(vertexBuffer.Swigcptr()+uintptr(vertexOffsetPos)))
		gl.VertexAttribPointer(uint32(impl.attribLocationUV), 2, gl.FLOAT, false, int32(vertexSize), unsafe.Pointer(vertexBuffer.Swigcptr()+uintptr(vertexOffsetUv)))
		gl.VertexAttribPointer(uint32(impl.attribLocationColor), 4, gl.UNSIGNED_BYTE, true, int32(vertexSize), unsafe.Pointer(vertexBuffer.Swigcptr()+uintptr(vertexOffsetCol)))

		gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, impl.elementsHandle)
		gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, int(indexBufferSize)*int(indexSize), unsafe.Pointer(indexBuffer), gl.STREAM_DRAW)

		for i := int64(0); i < list.CommandsSize(); i++ {
			cmd := list.CommandsAt(i)
			if cmd == nil {
				continue
			}

			elemCount := cmd.GetElemCount()
			if cmd.GetUserCallback() != nil {
				if imgui.CallDrawCmdCallback(list, cmd) {
					gl.EnableVertexAttribArray(uint32(impl.attribLocationPosition))
					gl.EnableVertexAttribArray(uint32(impl.attribLocationUV))
					gl.EnableVertexAttribArray(uint32(impl.attribLocationColor))

					gl.VertexAttribPointer(uint32(impl.attribLocationPosition), 2, gl.FLOAT, false, int32(vertexSize), unsafe.Pointer(vertexBuffer.Swigcptr()+uintptr(vertexOffsetPos)))
					gl.VertexAttribPointer(uint32(impl.attribLocationUV), 2, gl.FLOAT, false, int32(vertexSize), unsafe.Pointer(vertexBuffer.Swigcptr()+uintptr(vertexOffsetUv)))
					gl.VertexAttribPointer(uint32(impl.attribLocationColor), 4, gl.UNSIGNED_BYTE, true, int32(vertexSize), unsafe.Pointer(vertexBuffer.Swigcptr()+uintptr(vertexOffsetCol)))

					gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, impl.elementsHandle)
					gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, int(indexBufferSize)*int(indexSize), unsafe.Pointer(indexBuffer), gl.STREAM_DRAW)
				}
			} else {
				cmdClipRect := cmd.GetClipRect()
				//log.Println("\tclip:", j, i, cmdClipRect.GetX(), cmdClipRect.GetY(), cmdClipRect.GetZ(), cmdClipRect.GetW())
				clipRectX, clipRectY, clipRectZ, clipRectW :=
					cmdClipRect.GetX()-displayPos.GetX(),
					cmdClipRect.GetY()-displayPos.GetY(),
					cmdClipRect.GetZ()-displayPos.GetX(),
					cmdClipRect.GetW()-displayPos.GetY()
				if int(clipRectX) < fbWidth && int(clipRectY) < fbHeight && clipRectZ >= 0.0 && clipRectW >= 0.0 {
					// Apply scissor/clipping rectangle
					gl.Scissor(int32(clipRectX), int32(fbHeight)-int32(clipRectW), int32(clipRectZ-clipRectX), int32(clipRectW-clipRectY))

					// Bind texture, Draw
					gl.BindTexture(gl.TEXTURE_2D, uint32(cmd.GetTextureId()))
					gl.DrawElements(gl.TRIANGLES, int32(elemCount), uint32(drawType), unsafe.Pointer(uintptr(indexBufferOffset)))
				}
			}
			indexBufferOffset += elemCount * uint(indexSize)
		}
	}
	//gl.DeleteVertexArrays(1, &vaoHandle)

	// Restore modified GL state
	gl.UseProgram(uint32(lastProgram))
	gl.BindTexture(gl.TEXTURE_2D, uint32(lastTexture))
	gl.ActiveTexture(uint32(lastActiveTexture))
	//gl.BindVertexArray(uint32(lastVertexArray))
	gl.BindBuffer(gl.ARRAY_BUFFER, uint32(lastArrayBuffer))
	gl.BlendEquationSeparate(uint32(lastBlendEquationRgb), uint32(lastBlendEquationAlpha))
	gl.BlendFuncSeparate(uint32(lastBlendSrcRgb), uint32(lastBlendDstRgb), uint32(lastBlendSrcAlpha), uint32(lastBlendDstAlpha))
	if lastEnableBlend {
		gl.Enable(gl.BLEND)
	} else {
		gl.Disable(gl.BLEND)
	}
	if lastEnableCullFace {
		gl.Enable(gl.CULL_FACE)
	} else {
		gl.Disable(gl.CULL_FACE)
	}
	if lastEnableDepthTest {
		gl.Enable(gl.DEPTH_TEST)
	} else {
		gl.Disable(gl.DEPTH_TEST)
	}
	if lastEnableScissorTest {
		gl.Enable(gl.SCISSOR_TEST)
	} else {
		gl.Disable(gl.SCISSOR_TEST)
	}
	gl.Viewport(lastViewport[0], lastViewport[1], lastViewport[2], lastViewport[3])
	gl.Scissor(lastScissorBox[0], lastScissorBox[1], lastScissorBox[2], lastScissorBox[3])
}

func (impl *Render) createFontsTexture() {
	// Build texture atlas
	io := imgui.GetIO()
	Pixels, Width, Height := io.GetFonts().GetTexDataAsRGBA32()

	// Upload texture to graphics system
	var lastTexture int32
	gl.GetIntegerv(gl.TEXTURE_BINDING_2D, &lastTexture)
	gl.GenTextures(1, &impl.fontTexture)
	gl.BindTexture(gl.TEXTURE_2D, impl.fontTexture)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	//gl.PixelStorei(gl.UNPACK_ROW_LENGTH, 0)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(Width), int32(Height),
		0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(Pixels))

	// Store our identifier
	io.GetFonts().SetTexID(uintptr(impl.fontTexture))

	// Restore state
	gl.BindTexture(gl.TEXTURE_2D, uint32(lastTexture))
}

func (impl *Render) destroyFontsTexture() {
	if impl.fontTexture != 0 {
		gl.DeleteTextures(1, &impl.fontTexture)
		impl.fontTexture = 0
		imgui.GetIO().GetFonts().SetTexID(0)
	}
}

// If you get an error please report on github. You may try different GL context version or GLSL version.
func checkShader(handle uint32, desc string) bool {
	status, logLength := int32(0), int32(0)
	gl.GetShaderiv(handle, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		gl.GetShaderiv(handle, gl.INFO_LOG_LENGTH, &logLength)
		logStr := ""
		if logLength > 0 {
			buf := make([]byte, logLength+1)
			gl.GetShaderInfoLog(handle, logLength+1, &logLength, (*uint8)(gl.Ptr(&buf[0])))
			logStr = string(buf[:logLength])
		}
		log.Println("ERROR: ", desc, ":", logStr)
	}
	return status == gl.TRUE
}

// If you get an error please report on github. You may try different GL context version or GLSL version.
func checkProgram(handle uint32, desc string) bool {
	status, logLength := int32(0), int32(0)
	gl.GetProgramiv(handle, gl.LINK_STATUS, &status)

	if status == gl.FALSE {
		gl.GetProgramiv(handle, gl.INFO_LOG_LENGTH, &logLength)
		logStr := ""
		if logLength > 0 {
			buf := make([]byte, logLength+1)
			gl.GetProgramInfoLog(handle, logLength, &logLength, (*uint8)(gl.Ptr(&buf[0])))
			logStr = string(buf[:logLength])
		}
		log.Println("ERROR: ", desc, ":", logStr)
	}
	return status == gl.TRUE
}

func (impl *Render) CreateDeviceObjects() {
	// Backup GL state
	var lastTexture int32
	var lastArrayBuffer int32
	//var lastVertexArray int32
	gl.GetIntegerv(gl.TEXTURE_BINDING_2D, &lastTexture)
	gl.GetIntegerv(gl.ARRAY_BUFFER_BINDING, &lastArrayBuffer)
	//gl.GetIntegerv(gl.VERTEX_ARRAY_BINDING, &lastVertexArray)

	// Parse GLSL version string
	glslVersion := 120
	fmt.Sscanf(impl.glslVersion, "#version %d", &glslVersion)

	const vertex_shader_glsl_120 = "uniform mat4 ProjMtx;\n" +
		"attribute vec2 Position;\n" +
		"attribute vec2 UV;\n" +
		"attribute vec4 Color;\n" +
		"varying vec2 Frag_UV;\n" +
		"varying vec4 Frag_Color;\n" +
		"void main()\n" +
		"{\n" +
		"    Frag_UV = UV;\n" +
		"    Frag_Color = Color;\n" +
		"    gl_Position = ProjMtx * vec4(Position.xy,0,1);\n" +
		"}\n"

	const vertex_shader_glsl_130 = "uniform mat4 ProjMtx;\n" +
		"in vec2 Position;\n" +
		"in vec2 UV;\n" +
		"in vec4 Color;\n" +
		"out vec2 Frag_UV;\n" +
		"out vec4 Frag_Color;\n" +
		"void main()\n" +
		"{\n" +
		"    Frag_UV = UV;\n" +
		"    Frag_Color = Color;\n" +
		"    gl_Position = ProjMtx * vec4(Position.xy,0,1);\n" +
		"}\n"

	const vertex_shader_glsl_300_es = "precision mediump float;\n" +
		"layout (location = 0) in vec2 Position;\n" +
		"layout (location = 1) in vec2 UV;\n" +
		"layout (location = 2) in vec4 Color;\n" +
		"uniform mat4 ProjMtx;\n" +
		"out vec2 Frag_UV;\n" +
		"out vec4 Frag_Color;\n" +
		"void main()\n" +
		"{\n" +
		"    Frag_UV = UV;\n" +
		"    Frag_Color = Color;\n" +
		"    gl_Position = ProjMtx * vec4(Position.xy,0,1);\n" +
		"}\n"

	const vertex_shader_glsl_410_core = "layout (location = 0) in vec2 Position;\n" +
		"layout (location = 1) in vec2 UV;\n" +
		"layout (location = 2) in vec4 Color;\n" +
		"uniform mat4 ProjMtx;\n" +
		"out vec2 Frag_UV;\n" +
		"out vec4 Frag_Color;\n" +
		"void main()\n" +
		"{\n" +
		"    Frag_UV = UV;\n" +
		"    Frag_Color = Color;\n" +
		"    gl_Position = ProjMtx * vec4(Position.xy,0,1);\n" +
		"}\n"

	const fragment_shader_glsl_120 = "#ifdef GL_ES\n" +
		"    precision mediump float;\n" +
		"#endif\n" +
		"uniform sampler2D Texture;\n" +
		"varying vec2 Frag_UV;\n" +
		"varying vec4 Frag_Color;\n" +
		"void main()\n" +
		"{\n" +
		"    gl_FragColor = Frag_Color * texture2D(Texture, Frag_UV.st);\n" +
		"}\n"

	const fragment_shader_glsl_130 = "uniform sampler2D Texture;\n" +
		"in vec2 Frag_UV;\n" +
		"in vec4 Frag_Color;\n" +
		"out vec4 Out_Color;\n" +
		"void main()\n" +
		"{\n" +
		"    Out_Color = Frag_Color * texture(Texture, Frag_UV.st);\n" +
		"}\n"

	const fragment_shader_glsl_300_es = "precision mediump float;\n" +
		"uniform sampler2D Texture;\n" +
		"in vec2 Frag_UV;\n" +
		"in vec4 Frag_Color;\n" +
		"layout (location = 0) out vec4 Out_Color;\n" +
		"void main()\n" +
		"{\n" +
		"    Out_Color = Frag_Color * texture(Texture, Frag_UV.st);\n" +
		"}\n"

	const fragment_shader_glsl_410_core = "in vec2 Frag_UV;\n" +
		"in vec4 Frag_Color;\n" +
		"uniform sampler2D Texture;\n" +
		"layout (location = 0) out vec4 Out_Color;\n" +
		"void main()\n" +
		"{\n" +
		"    Out_Color = Frag_Color * texture(Texture, Frag_UV.st);\n" +
		"}\n"

	// Select shaders matching our GLSL versions
	vertexShader, fragmentShader := "", ""
	if glslVersion < 130 {
		vertexShader = vertex_shader_glsl_120
		fragmentShader = fragment_shader_glsl_120
	} else if glslVersion == 410 {
		vertexShader = vertex_shader_glsl_410_core
		fragmentShader = fragment_shader_glsl_410_core
	} else if glslVersion == 300 {
		vertexShader = vertex_shader_glsl_300_es
		fragmentShader = fragment_shader_glsl_300_es
	} else {
		vertexShader = vertex_shader_glsl_130
		fragmentShader = fragment_shader_glsl_130
	}

	// Create shaders
	impl.vertHandle = gl.CreateShader(gl.VERTEX_SHADER)
	vstrs, free1 := gl.Strs(impl.glslVersion+"\n\x00", vertexShader+"\x00")
	defer free1()
	gl.ShaderSource(impl.vertHandle, 2, vstrs, nil)
	gl.CompileShader(impl.vertHandle)
	checkShader(impl.vertHandle, "vertex shader")

	impl.fragHandle = gl.CreateShader(gl.FRAGMENT_SHADER)
	fstrs, free2 := gl.Strs(impl.glslVersion+"\n\x00", fragmentShader+"\x00")
	defer free2()
	gl.ShaderSource(impl.fragHandle, 2, fstrs, nil)
	gl.CompileShader(impl.fragHandle)
	checkShader(impl.fragHandle, "fragment shader")

	impl.shaderHandle = gl.CreateProgram()
	gl.AttachShader(impl.shaderHandle, impl.vertHandle)
	gl.AttachShader(impl.shaderHandle, impl.fragHandle)
	gl.LinkProgram(impl.shaderHandle)
	checkProgram(impl.shaderHandle, "shader program")

	impl.attribLocationTex = gl.GetUniformLocation(impl.shaderHandle, gl.Str("Texture"+"\x00"))
	impl.attribLocationProjMtx = gl.GetUniformLocation(impl.shaderHandle, gl.Str("ProjMtx"+"\x00"))
	impl.attribLocationPosition = gl.GetAttribLocation(impl.shaderHandle, gl.Str("Position"+"\x00"))
	impl.attribLocationUV = gl.GetAttribLocation(impl.shaderHandle, gl.Str("UV"+"\x00"))
	impl.attribLocationColor = gl.GetAttribLocation(impl.shaderHandle, gl.Str("Color"+"\x00"))

	//gl.GenBuffers(1, &impl.vboHandle)
	gl.GenBuffers(1, &impl.elementsHandle)

	impl.createFontsTexture()

	// Restore modified GL state
	gl.BindTexture(gl.TEXTURE_2D, uint32(lastTexture))
	gl.BindBuffer(gl.ARRAY_BUFFER, uint32(lastArrayBuffer))
	//gl.BindVertexArray(uint32(lastVertexArray))
}

func (impl *Render) DestroyDeviceObjects() {
	//if impl.vboHandle != 0 {
	//	gl.DeleteBuffers(1, &impl.vboHandle)
	//}
	//impl.vboHandle = 0
	if impl.elementsHandle != 0 {
		gl.DeleteBuffers(1, &impl.elementsHandle)
	}
	impl.elementsHandle = 0

	if (impl.shaderHandle != 0) && (impl.vertHandle != 0) {
		gl.DetachShader(impl.shaderHandle, impl.vertHandle)
	}
	if impl.vertHandle != 0 {
		gl.DeleteShader(impl.vertHandle)
	}
	impl.vertHandle = 0

	if (impl.shaderHandle != 0) && (impl.fragHandle != 0) {
		gl.DetachShader(impl.shaderHandle, impl.fragHandle)
	}
	if impl.fragHandle != 0 {
		gl.DeleteShader(impl.fragHandle)
	}
	impl.fragHandle = 0

	if impl.shaderHandle != 0 {
		gl.DeleteProgram(impl.shaderHandle)
	}
	impl.shaderHandle = 0

	impl.destroyFontsTexture()
}
