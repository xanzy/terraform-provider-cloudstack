package cloudstack

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/terraform"
	"github.com/xanzy/go-cloudstack/cloudstack"
)

func TestAccCloudStackProject_basic(t *testing.T) {
	var project cloudstack.Project

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckCloudStackProjectDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudStackProject_basic,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudStackProjectExists("cloudstack_project.foo", &project),
					testAccCheckCloudStackProjectAttributes(&project),
				),
			},
		},
	})
}

func TestAccCloudStackProject_import(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckCloudStackProjectDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudStackProject_basic,
			},

			{
				ResourceName:      "cloudstack_project.foo",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckCloudStackProjectExists(
	n string, project *cloudstack.Project) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No project ID is set")
		}

		cs := testAccProvider.Meta().(*cloudstack.CloudStackClient)

		p, _, err := cs.Project.GetProjectByID(
			rs.Primary.ID,
			cloudstack.WithDomain(rs.Primary.Attributes["domain_id"]),
		)
		if err != nil {
			return err
		}

		if p.Id != rs.Primary.ID {
			return fmt.Errorf("Project not found")
		}

		*project = *p

		return nil
	}
}

func testAccCheckCloudStackProjectAttributes(
	project *cloudstack.Project) resource.TestCheckFunc {
	return func(s *terraform.State) error {

		if project.Name != "terraform-project" {
			return fmt.Errorf("Bad name: %s", project.Name)
		}

		if project.Displaytext != "terraform-project-text" {
			return fmt.Errorf("Bad display text: %s", project.Displaytext)
		}

		return nil
	}
}

func testAccCheckCloudStackProjectDestroy(s *terraform.State) error {
	cs := testAccProvider.Meta().(*cloudstack.CloudStackClient)

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "cloudstack_project" {
			continue
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No Project ID is set")
		}

		_, _, err := cs.Project.GetProjectByID(rs.Primary.ID)
		if err == nil {
			return fmt.Errorf("Project %s still exists", rs.Primary.ID)
		}
	}

	return nil
}

const testAccCloudStackProject_basic = `
resource "cloudstack_project" "foo" {
  name = "terraform-project"
  display_text = "terraform-project-text"
}`
