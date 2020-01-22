const assert = require('assert');
const crypto = require('crypto');
const { COMPOSITE_SEPARATOR, HASH_KEY_SEPARATOR, ASCENDING_KEY_PADDING } = require('./constants');

class StringKey {
  constructor(mapping, key) {
    assert(mapping[key], `key ${key} is not defined in mapping`);

    this.key = key;
    this.mapping = mapping;
    this.type = mapping[this.key];
    this.covers = [key];
  }

  exact(properties) {
    // Get value
    const value = properties[this.key];
    // Check that value was given
    assert(value !== undefined, 'Unable to create key from properties');
    // Return exact key
    return this.type.string(value);
  }
}

class DescendingIntegerKey {
  constructor(mapping, key) {
    assert(mapping[key], `key ${key} is not defined in mapping`);

    this.key = key;
    this.type = mapping[key];
    this.covers = [key];
  }
}

class AscendingIntegerKey {
  constructor(mapping, key) {
    assert(mapping[key], `key ${key} is not defined in mapping`);
    // TODO: Add the second assertion (see azure-entities)

    this.key = key;
    this.type = mapping[key];
    this.covers = [key];
  }

  exact(properties) {
    // Get value
    const value = properties[this.key];
    // Check that value was given
    assert(value !== undefined, 'Unable to create key from properties');
    // Return exact key
    const str = value.toString();

    return ASCENDING_KEY_PADDING.substring(
      0, ASCENDING_KEY_PADDING.length - str.length,
    ) + str;
  }
}

class ConstantKey {
  constructor(constant) {
    assert.equal(typeof constant, 'string', 'ConstantKey takes a string');

    this.constant = constant;
    this.covers = [];
  }
}

class CompositeKey {
  constructor(mapping, keys) {
    assert(keys instanceof Array, 'keys must be an array');
    assert(keys.length > 0, 'CompositeKey needs at least one key');

    this.keys = keys;
    this.types = [];

    for (let i = 0; i < keys.length; i++) {
      assert(mapping[keys[i]], `key ${keys[i]} is not defined in mapping`);
      this.types[i] = mapping[keys[i]];
    }

    this.covers = keys;
  }

  exact(properties) {
    // Map each key to it's string encoded value
    return this.keys.map(function(key, index) {
      // Get value from key
      const value = properties[key];

      if (value === undefined) {
        throw new Error(`Unable to render CompositeKey from properties, missing ${key}`);
      }

      return this.types[index].string(value);
    }, this).join(COMPOSITE_SEPARATOR); // Join with separator
  }
}

class HashKey {
  constructor(mapping, keys) {
    assert(keys instanceof Array, 'keys must be an array');
    assert(keys.length > 0, 'HashKey needs at least one key');

    this.keys = keys;
    this.mapping = mapping;
    this.covers = keys;
    this.types = [];

    for (let i = 0; i < keys.length; i++) {
      assert(mapping[keys[i]], `key ${keys[i]} is not defined in mapping`);
      this.types[i] = mapping[keys[i]];
    }
  }

  exact(properties) {
    const hash =  crypto.createHash('sha512');
    const n = this.keys.length;

    for (let i = 0; i < n; i++) {
      const key = this.keys[i];

      // Get value from key
      const value = properties[key];

      if (value === undefined) {
        throw new Error(`Unable to render HashKey from properties, missing ${key}`);
      }

      // Find hash value and update the hashsum
      hash.update(this.types[i].hash(value), 'utf8');

      // Insert separator, if this isn't the last key
      if (i + 1 < n) {
        hash.update(HASH_KEY_SEPARATOR, 'utf8');
      }
    }

    return hash.digest('hex');
  }
}

module.exports = {
  StringKey: function(key) {
    const keys = [...arguments];

    assert.equal(keys.length, 1, 'StringKey takes exactly one key argument');
    assert.equal(typeof key, 'string', 'StringKey takes a string as argument');

    return mapping => new StringKey(mapping, key);
  },
  DescendingIntegerKey: function(key) {
    return mapping => new DescendingIntegerKey(mapping, key);
  },
  AscendingIntegerKey: function(key) {
    return mapping => new AscendingIntegerKey(mapping, key);
  },
  ConstantKey: function(constant) {
    assert.equal(typeof constant, 'string', `ConstantKey takes a string`);
    return mapping => new ConstantKey(constant);
  },
  CompositeKey: function() {
    const keys = [...arguments];

    keys.forEach(function(key) {
      assert.equal(typeof key, 'string', 'CompositeKey takes strings as arguments');
    });

    return mapping => new CompositeKey(mapping, keys);
  },
  HashKey: function() {
    const keys = [...arguments];

    keys.forEach(function(key) {
      assert.equal(typeof key, 'string', 'HashKey takes strings as arguments');
    });

    return mapping => new HashKey(mapping, keys);
  },
};