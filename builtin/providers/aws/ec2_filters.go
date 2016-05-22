package aws

import (
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/hashicorp/terraform/helper/schema"
)

func buildEC2FilterList(
	d *schema.ResourceData,
	attrs map[string]string,
) []*ec2.Filter {

	var customFilters []interface{}
	if filterSetI, ok := d.GetOk("filter"); ok {
		customFilters = filterSetI.(*schema.Set).List()
	}

	var tags map[string]interface{}
	if tagsI, ok := d.GetOk("tags"); ok {
		tags = tagsI.(map[string]interface{})
	}

	filterCount := len(attrs) + len(customFilters) + len(tags)
	filters := make([]*ec2.Filter, 0, filterCount)

	for attrName, filterName := range attrs {
		if valI, ok := d.GetOk(attrName); ok {
			var val string

			switch valI.(type) {
			case string:
				val = valI.(string)
			case bool:
				if valB := valI.(bool); valB {
					val = "true"
				} else {
					val = "false"
				}
			case int:
				val = strconv.Itoa(valI.(int))
			default:
				panic(fmt.Errorf("Unsupported filter type %#v", valI))
			}

			filters = append(filters, &ec2.Filter{
				Name:   aws.String(filterName),
				Values: []*string{&val},
			})
		}
	}

	for _, customFilterI := range customFilters {
		customFilterMapI := customFilterI.(map[string]interface{})
		name := customFilterMapI["name"].(string)
		valuesI := customFilterMapI["values"].(*schema.Set).List()
		values := make([]*string, len(valuesI))
		for i, valueI := range valuesI {
			values[i] = aws.String(valueI.(string))
		}

		filters = append(filters, &ec2.Filter{
			Name:   &name,
			Values: values,
		})
	}

	for name, valueI := range tags {
		filters = append(filters, &ec2.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", name)),
			Values: []*string{aws.String(valueI.(string))},
		})
	}

	if len(filters) == 0 {
		return nil
	}

	return filters
}
