import { api } from "api/api";

export const deploymentConfig = () => {
  return {
    queryKey: ["deployment", "config"],
    queryFn: api.getDeploymentConfig,
  };
};

export const deploymentDAUs = () => {
  return {
    queryKey: ["deployment", "daus"],
    queryFn: api.getDeploymentDAUs,
  };
};

export const deploymentStats = () => {
  return {
    queryKey: ["deployment", "stats"],
    queryFn: api.getDeploymentStats,
  };
};

export const deploymentSSHConfig = () => {
  return {
    queryKey: ["deployment", "sshConfig"],
    queryFn: api.getDeploymentSSHConfig,
  };
};
