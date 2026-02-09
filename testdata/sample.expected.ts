// Sample file with various import assertion patterns.

import data from './data.json' with { type: 'json' };
import { config } from '../config.json' with { type: 'json' };
import * as translations from './i18n/en.json' with { type: 'json' };
import styles from './component.css' with { type: 'css' };

// Re-exports
export { default as schema } from './schema.json' with { type: 'json' };
export { version } from './package.json' with { type: 'json' };

// Regular imports (should not be changed)
import React from 'react';
import { useState } from 'react';

// Already using `with` (should not be changed)
import manifest from './manifest.json' with { type: 'json' };

console.log(data, config, translations, styles);
