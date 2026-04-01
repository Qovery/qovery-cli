package utils

import (
	"github.com/qovery/qovery-client-go"
)

// ConvertAutoscalingResponseToRequest converts an AutoscalingPolicyResponse (from the API)
// into an AutoscalingPolicyRequest suitable for update calls, preserving existing KEDA config.
func ConvertAutoscalingResponseToRequest(resp *qovery.AutoscalingPolicyResponse) *qovery.AutoscalingPolicyRequest {
	if resp == nil || resp.KedaAutoscalingResponse == nil {
		return nil
	}

	kedaResp := resp.KedaAutoscalingResponse

	var scalers []qovery.KedaScalerRequest
	for _, s := range kedaResp.Scalers {
		enabled := s.Enabled
		scaler := qovery.KedaScalerRequest{
			ScalerType: s.ScalerType,
			Enabled:    &enabled,
			Role:       s.Role,
			ConfigJson: s.ConfigJson,
			ConfigYaml: s.ConfigYaml.Get(),
		}
		if s.TriggerAuthentication != nil {
			scaler.TriggerAuthentication = &qovery.KedaTriggerAuthenticationRequest{
				Name:       s.TriggerAuthentication.Name,
				ConfigYaml: s.TriggerAuthentication.ConfigYaml,
			}
		}
		scalers = append(scalers, scaler)
	}

	kedaReq := &qovery.KedaAutoscalingRequest{
		Mode:                   kedaResp.Mode,
		PollingIntervalSeconds: &kedaResp.PollingIntervalSeconds,
		CooldownPeriodSeconds:  &kedaResp.CooldownPeriodSeconds,
		Scalers:                scalers,
	}

	result := qovery.KedaAutoscalingRequestAsAutoscalingPolicyRequest(kedaReq)
	return &result
}
