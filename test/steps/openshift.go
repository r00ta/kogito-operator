// Copyright 2020 Red Hat, Inc. and/or its affiliates
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package steps

import (
	"github.com/cucumber/godog"
	"github.com/kiegroup/kogito-cloud-operator/test/framework"
	"github.com/kiegroup/kogito-cloud-operator/test/steps/mappers"
	v1 "k8s.io/api/core/v1"
)

/*
	DataTable for BuildConfig build resources:
	| build-request  | cpu/memory     | value  |
	| build-limit    | cpu/memory     | value  |
*/

func registerOpenShiftSteps(ctx *godog.ScenarioContext, data *Data) {
	// Build steps
	ctx.Step(`^Start build with name "([^"]*)" from local example service path "([^"]*)"$`, data.startBuildFromExampleServicePath)
	ctx.Step(`^Start build with name "([^"]*)" from local example service file "([^"]*)"$`, data.startBuildFromExampleServiceFile)
	ctx.Step(`^Build "([^"]*)" is complete after (\d+) minutes$`, data.buildIsCompleteAfterMinutes)

	// BuildConfig steps
	ctx.Step(`^BuildConfig "([^"]*)" is created after (\d+) minutes$`, data.buildConfigIsCreatedAfterMinutes)
	ctx.Step(`^BuildConfig "([^"]*)" is created with build resources within (\d+) minutes:$`, data.buildConfigHasResourcesWithinMinutes)
	ctx.Step(`^BuildConfig "([^"]*)" is created with webhooks within (\d+) minutes$`, data.buildConfigHasWebhooksWithinMinutes)
}

// Build steps
func (data *Data) startBuildFromExampleServicePath(buildName, localExamplePath string) error {
	examplesRepositoryPath := data.KogitoExamplesLocation
	_, err := framework.CreateCommand("oc", "start-build", buildName, "--from-dir="+examplesRepositoryPath+"/"+localExamplePath, "-n", data.Namespace).WithLoggerContext(data.Namespace).Execute()
	return err
}

func (data *Data) startBuildFromExampleServiceFile(buildName, localExampleFilePath string) error {
	examplesRepositoryPath := data.KogitoExamplesLocation
	_, err := framework.CreateCommand("oc", "start-build", buildName, "--from-file="+examplesRepositoryPath+"/"+localExampleFilePath, "-n", data.Namespace).WithLoggerContext(data.Namespace).Execute()
	return err
}

func (data *Data) buildIsCompleteAfterMinutes(buildName string, timeoutInMin int) error {
	return framework.WaitForBuildComplete(data.Namespace, buildName, timeoutInMin)
}

func (data *Data) buildConfigIsCreatedAfterMinutes(buildConfigName string, timeoutInMin int) error {
	return framework.WaitForBuildConfigCreated(data.Namespace, buildConfigName, timeoutInMin)
}

func (data *Data) buildConfigHasResourcesWithinMinutes(buildConfigName string, timeoutInMin int, dt *godog.Table) error {
	build := &v1.ResourceRequirements{Limits: v1.ResourceList{}, Requests: v1.ResourceList{}}
	err := mappers.MapBuildResourceRequirementsTable(dt, build)

	if err != nil {
		return err
	}

	return framework.WaitForBuildConfigToHaveResources(data.Namespace, buildConfigName, *build, timeoutInMin)
}

func (data *Data) buildConfigHasWebhooksWithinMinutes(buildConfigName string, timeoutInMin int) error {
	kogitoBuild, err := framework.GetKogitoBuild(data.Namespace, buildConfigName)

	if err != nil {
		return err
	}

	return framework.WaitForBuildConfigCreatedWithWebhooks(data.Namespace, buildConfigName, kogitoBuild.Spec.WebHooks, timeoutInMin)
}
