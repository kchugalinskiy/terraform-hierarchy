package main

import (
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestRegexUtils(t *testing.T) {
	Convey("Regex must work", t, func() {
		result := findAllVariables("${concat(var.input_1, module.blabla.id, var.input2, aws_instance.first.ip, var.input-3)}")
		So(len(result), ShouldEqual, 3)
		So(result[0], ShouldEqual, "input_1")
		So(result[1], ShouldEqual, "input2")
		So(result[2], ShouldEqual, "input-3")

		result2 := findAllResourceFields("${concat(aws_instance.second.ip, var.input_1, module.blabla.id, var.input2, aws_instance.first.ip, var.input-3)}")
		So(len(result2), ShouldEqual, 2)
		So(result2[0].Name, ShouldEqual, "aws_instance")
		So(result2[0].InstanceName, ShouldEqual, "second")
		So(result2[0].FieldName, ShouldEqual, "ip")
		So(result2[1].Name, ShouldEqual, "aws_instance")
		So(result2[1].InstanceName, ShouldEqual, "first")
		So(result2[1].FieldName, ShouldEqual, "ip")

		result3 := findAllModuleFields("${concat(aws_instance.second.ip, var.input_1, module.blabla.id, var.input2, aws_instance.first.ip, var.input-3, module.blabla2.id)}")
		So(len(result3), ShouldEqual, 2)
		So(result3[0].InstanceName, ShouldEqual, "blabla")
		So(result3[0].FieldName, ShouldEqual, "id")
		So(result3[1].InstanceName, ShouldEqual, "blabla2")
		So(result3[1].FieldName, ShouldEqual, "id")
	})
}
