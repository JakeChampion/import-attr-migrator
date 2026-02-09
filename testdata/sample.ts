// Sample file with various import assertion patterns.

import data from './data.json' assert { type: 'json' };
import { config } from '../config.json' assert { type: 'json' };
import * as translations from './i18n/en.json' assert { type: 'json' };
import styles from './component.css' assert { type: 'css' };

// Re-exports
export { default as schema } from './schema.json' assert { type: 'json' };
export { version } from './package.json' assert { type: 'json' };

// Regular imports (should not be changed)
import React from 'react';
import { useState } from 'react';

// Already using `with` (should not be changed)
import manifest from './manifest.json' with { type: 'json' };

console.log(data, config, translations, styles);
