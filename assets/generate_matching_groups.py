#!/usr/bin/env python3
import re
import sys


# echo を想定した実装
# group は考慮しない
def extract_route_from_line(line):
    """
    ソースコードの行から、ルーティングのパスを抽出する
    """
    methods = ['GET', 'POST', 'PUT', 'DELETE']
    # ルーティングに関する行か?
    if not any(f'.{method}(' in line for method in methods):
        return None

    # 第一引数を抽出
    # ダブルクォートの中身を抽出でよい?
    # e.GET("/api/users/:user_id", handler)
    route_match = re.search(r'".*"', line)
    if route_match is None:
        return None

    route = route_match.group(0)
    route = route.strip('"')

    return route


def extract_routes(input_stream):
    routes = []
    memo = set()
    for line in input_stream:
        line = line.strip()
        if not line:
            continue

        route = extract_route_from_line(line)
        # GET, POST で同じ route があり得る
        if route is not None and route not in memo:
            memo.add(route)
            routes.append(route)

    return routes


def create_route_regex_list(routes):
    route_regex_list = []
    no_param_routes = [route for route in routes if not contain_params(route)]
    for i, route in enumerate(routes):
        if not contain_params(route):
            continue

        route_regex = replace_params(route)

        # /api/users/:user_id と /api/users/me が区別できない
        # 後者のパターンを先にマッチさせる
        for r in no_param_routes:
            if re.match(route_regex, r):
                route_regex_list.append(f'^{r}$')
                # 念の為, 既に追加したものは消す
                no_param_routes.remove(r)

        route_regex_list.append(route_regex)

    return route_regex_list


def replace_params(route):
    return '^' + re.sub(r'/:[^/]+', '/[^/]+', route) + '$'


def contain_params(route):
    """
    ルーティングのパスにパラメータが含まれているか
    """
    return ':' in route


def print_as_yaml_list(route_regex_list):
    """
    alp の matching_groups 用の文字列を出力する
    """
    print('matching_groups:')
    for route_regex in route_regex_list:
        print(f'  - {route_regex}')


def main():
    """
    ソースコードを解析して、alp の matching_groups 用の文字列を出力する
    """
    if len(sys.argv) == 2:
        filename = sys.argv[1]
        with open(filename, 'r') as f:
            routes = extract_routes(f)
    elif len(sys.argv) == 1:
        routes = extract_routes(sys.stdin)
    else:
        print('Usage: python3 generate_matching_groups.py [filename]')
        sys.exit(1)

    route_regex_list = create_route_regex_list(routes)
    print_as_yaml_list(route_regex_list)


if __name__ == '__main__':
    main()
