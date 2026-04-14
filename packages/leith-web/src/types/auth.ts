export const USER_LEVELS = {
  0: { level: 0, label: '訪客', weight: 0, authMethod: 'oauth' },
  1: { level: 1, label: '認證人類', weight: 1, authMethod: 'passkey' },
  2: { level: 2, label: '活躍公民', weight: 5, authMethod: 'vouching' },
  3: { level: 3, label: '資深貢獻者', weight: 20, authMethod: 'time' },
  4: { level: 4, label: '權威實體', weight: 100, authMethod: 'vc' },
};

export interface User {
  address: string;
  displayName?: string;
  avatar?: string;
  joinedAt: number;
  verified?: boolean;
  level: 0 | 1 | 2 | 3 | 4;
}
