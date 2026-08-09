export interface Permission {
  action: string;
  resource: string;
}

export interface Role {
  id: string;
  name: string;
  description: string;
  permissions: Permission[];
  isBuiltIn?: boolean;
  createdAt?: string;
  updatedAt?: string;
}

export interface UserRoleAssignment {
  username: string;
  roleIds: string[];
  displayName?: string;
  email?: string;
  providerType?: string;
  providerId?: string;
  lastLoginAt?: string;
  updatedAt?: string;
}
