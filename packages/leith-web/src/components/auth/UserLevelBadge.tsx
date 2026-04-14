import React from 'react';
import { USER_LEVELS } from '../../types/auth';

interface UserLevelBadgeProps {
    level: 0 | 1 | 2 | 3 | 4;
    showLabel?: boolean;
}

export function UserLevelBadge({ level, showLabel = false }: UserLevelBadgeProps) {
    const info = USER_LEVELS[level];

    // Map levels to visual styles mimicking the prototype
    const getLevelStyles = () => {
        switch (level) {
            case 4:
                return 'bg-purple-500/20 text-purple-400 border border-purple-500/30';
            case 3:
                return 'bg-blue-500/20 text-blue-400 border border-blue-500/30';
            case 2:
                return 'bg-green-500/20 text-green-400 border border-green-500/30';
            case 1:
                return 'bg-[#ea580c]/20 text-[#ea580c] border border-[#ea580c]/30';
            case 0:
            default:
                return 'bg-neutral-800 text-neutral-400 border border-neutral-700';
        }
    };

    const renderIcon = () => {
        switch (level) {
            case 4:
                return <span>👑</span>;
            case 3:
                return <span>⭐</span>;
            case 2:
                return <span>✓</span>;
            case 1:
                return <span>🔑</span>;
            case 0:
            default:
                return <span>👁️</span>;
        }
    };

    return (
        <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-mono font-bold whitespace-nowrap ${getLevelStyles()}`}>
            {renderIcon()}
            {showLabel ? info.label : `L${level}`}
        </span>
    );
}
