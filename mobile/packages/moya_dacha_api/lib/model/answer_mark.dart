//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class AnswerMark {
  /// Returns a new [AnswerMark] instance.
  AnswerMark({
    required this.commentId,
  });

  /// Идентификатор комментария под этим постом (UUID)
  String commentId;

  @override
  bool operator ==(Object other) => identical(this, other) || other is AnswerMark &&
    other.commentId == commentId;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (commentId.hashCode);

  @override
  String toString() => 'AnswerMark[commentId=$commentId]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'comment_id'] = this.commentId;
    return json;
  }

  /// Returns a new [AnswerMark] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static AnswerMark? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'comment_id'), 'Required key "AnswerMark[comment_id]" is missing from JSON.');
        assert(json[r'comment_id'] != null, 'Required key "AnswerMark[comment_id]" has a null value in JSON.');
        return true;
      }());

      return AnswerMark(
        commentId: mapValueOfType<String>(json, r'comment_id')!,
      );
    }
    return null;
  }

  static List<AnswerMark> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <AnswerMark>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = AnswerMark.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, AnswerMark> mapFromJson(dynamic json) {
    final map = <String, AnswerMark>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = AnswerMark.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of AnswerMark-objects as value to a dart map
  static Map<String, List<AnswerMark>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<AnswerMark>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = AnswerMark.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'comment_id',
  };
}

