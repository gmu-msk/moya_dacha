//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class TagSuggestions {
  /// Returns a new [TagSuggestions] instance.
  TagSuggestions({
    this.items = const [],
  });

  /// Подсказанные тэги, лучшие первыми
  List<String> items;

  @override
  bool operator ==(Object other) => identical(this, other) || other is TagSuggestions &&
    _deepEquality.equals(other.items, items);

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (items.hashCode);

  @override
  String toString() => 'TagSuggestions[items=$items]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'items'] = this.items;
    return json;
  }

  /// Returns a new [TagSuggestions] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static TagSuggestions? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'items'), 'Required key "TagSuggestions[items]" is missing from JSON.');
        assert(json[r'items'] != null, 'Required key "TagSuggestions[items]" has a null value in JSON.');
        return true;
      }());

      return TagSuggestions(
        items: json[r'items'] is Iterable
            ? (json[r'items'] as Iterable).cast<String>().toList(growable: false)
            : const [],
      );
    }
    return null;
  }

  static List<TagSuggestions> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <TagSuggestions>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = TagSuggestions.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, TagSuggestions> mapFromJson(dynamic json) {
    final map = <String, TagSuggestions>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = TagSuggestions.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of TagSuggestions-objects as value to a dart map
  static Map<String, List<TagSuggestions>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<TagSuggestions>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = TagSuggestions.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'items',
  };
}

