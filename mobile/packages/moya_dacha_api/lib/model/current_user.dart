//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class CurrentUser {
  /// Returns a new [CurrentUser] instance.
  CurrentUser({
    required this.id,
    required this.nickname,
    required this.nicknameChosen,
    required this.phone,
    required this.createdAt,
    required this.name,
    required this.about,
    this.avatarUrl,
    required this.closed,
    this.place,
  });

  /// Идентификатор пользователя (UUID)
  String id;

  /// Уникальный никнейм: им пользователь подписан везде. Пока пользователь не выбрал его сам, он временный — `dachnik_…`. 
  String nickname;

  /// Выбран ли никнейм самим пользователем. Пока `false`, приложение показывает экран знакомства. 
  bool nicknameChosen;

  /// Нормализованный номер телефона
  String phone;

  /// Когда пользователь зарегистрировался
  DateTime createdAt;

  /// Полное имя, может быть пустым
  String name;

  /// Короткое «о себе», может быть пустым
  String about;

  /// Ссылка на аватар или `null`, если аватара нет. Может быть относительной — клиент достраивает её до адреса сервиса. 
  String? avatarUrl;

  /// Закрыт ли профиль: посты и списки подписок видят только подписчики, новые подписываются по заявке (specs/012-follows.md) 
  bool closed;

  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  Place? place;

  @override
  bool operator ==(Object other) => identical(this, other) || other is CurrentUser &&
    other.id == id &&
    other.nickname == nickname &&
    other.nicknameChosen == nicknameChosen &&
    other.phone == phone &&
    other.createdAt == createdAt &&
    other.name == name &&
    other.about == about &&
    other.avatarUrl == avatarUrl &&
    other.closed == closed &&
    other.place == place;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (id.hashCode) +
    (nickname.hashCode) +
    (nicknameChosen.hashCode) +
    (phone.hashCode) +
    (createdAt.hashCode) +
    (name.hashCode) +
    (about.hashCode) +
    (avatarUrl == null ? 0 : avatarUrl!.hashCode) +
    (closed.hashCode) +
    (place == null ? 0 : place!.hashCode);

  @override
  String toString() => 'CurrentUser[id=$id, nickname=$nickname, nicknameChosen=$nicknameChosen, phone=$phone, createdAt=$createdAt, name=$name, about=$about, avatarUrl=$avatarUrl, closed=$closed, place=$place]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'id'] = this.id;
      json[r'nickname'] = this.nickname;
      json[r'nickname_chosen'] = this.nicknameChosen;
      json[r'phone'] = this.phone;
      json[r'created_at'] = this.createdAt.toUtc().toIso8601String();
      json[r'name'] = this.name;
      json[r'about'] = this.about;
    if (this.avatarUrl != null) {
      json[r'avatar_url'] = this.avatarUrl;
    } else {
      json[r'avatar_url'] = null;
    }
      json[r'closed'] = this.closed;
    if (this.place != null) {
      json[r'place'] = this.place;
    } else {
      json[r'place'] = null;
    }
    return json;
  }

  /// Returns a new [CurrentUser] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static CurrentUser? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'id'), 'Required key "CurrentUser[id]" is missing from JSON.');
        assert(json[r'id'] != null, 'Required key "CurrentUser[id]" has a null value in JSON.');
        assert(json.containsKey(r'nickname'), 'Required key "CurrentUser[nickname]" is missing from JSON.');
        assert(json[r'nickname'] != null, 'Required key "CurrentUser[nickname]" has a null value in JSON.');
        assert(json.containsKey(r'nickname_chosen'), 'Required key "CurrentUser[nickname_chosen]" is missing from JSON.');
        assert(json[r'nickname_chosen'] != null, 'Required key "CurrentUser[nickname_chosen]" has a null value in JSON.');
        assert(json.containsKey(r'phone'), 'Required key "CurrentUser[phone]" is missing from JSON.');
        assert(json[r'phone'] != null, 'Required key "CurrentUser[phone]" has a null value in JSON.');
        assert(json.containsKey(r'created_at'), 'Required key "CurrentUser[created_at]" is missing from JSON.');
        assert(json[r'created_at'] != null, 'Required key "CurrentUser[created_at]" has a null value in JSON.');
        assert(json.containsKey(r'name'), 'Required key "CurrentUser[name]" is missing from JSON.');
        assert(json[r'name'] != null, 'Required key "CurrentUser[name]" has a null value in JSON.');
        assert(json.containsKey(r'about'), 'Required key "CurrentUser[about]" is missing from JSON.');
        assert(json[r'about'] != null, 'Required key "CurrentUser[about]" has a null value in JSON.');
        assert(json.containsKey(r'closed'), 'Required key "CurrentUser[closed]" is missing from JSON.');
        assert(json[r'closed'] != null, 'Required key "CurrentUser[closed]" has a null value in JSON.');
        return true;
      }());

      return CurrentUser(
        id: mapValueOfType<String>(json, r'id')!,
        nickname: mapValueOfType<String>(json, r'nickname')!,
        nicknameChosen: mapValueOfType<bool>(json, r'nickname_chosen')!,
        phone: mapValueOfType<String>(json, r'phone')!,
        createdAt: mapDateTime(json, r'created_at', r'')!,
        name: mapValueOfType<String>(json, r'name')!,
        about: mapValueOfType<String>(json, r'about')!,
        avatarUrl: mapValueOfType<String>(json, r'avatar_url'),
        closed: mapValueOfType<bool>(json, r'closed')!,
        place: Place.fromJson(json[r'place']),
      );
    }
    return null;
  }

  static List<CurrentUser> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <CurrentUser>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = CurrentUser.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, CurrentUser> mapFromJson(dynamic json) {
    final map = <String, CurrentUser>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = CurrentUser.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of CurrentUser-objects as value to a dart map
  static Map<String, List<CurrentUser>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<CurrentUser>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = CurrentUser.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'id',
    'nickname',
    'nickname_chosen',
    'phone',
    'created_at',
    'name',
    'about',
    'closed',
  };
}

