<template>
  <v-card>
    <v-card-title>Query format</v-card-title>
    <v-card-text>
      <v-simple-table>
        <tbody>
          <tr>
            <th>Operators</th>
            <td><code>filter&nbsp;[AND]|OR|THEN&nbsp;filter</code></td>
            <td width="100%">
              <code>AND</code>/<code>OR</code> do what you expect.
              <code>THEN</code> works like <code>AND</code> but makes
              <code>[cs]data</code> filters match sequentially.
              <code>AND</code> can be omitted.
            </td>
          </tr>
          <tr>
            <th>Brackets</th>
            <td><code>(filter)</code></td>
            <td width="100%">Filters can be grouped in brackets.</td>
          </tr>
          <tr>
            <th>Negation</th>
            <td><code>-filter</code></td>
            <td width="100%">Inverts the filter.</td>
          </tr>
          <tr>
            <th>Filter&nbsp;format</th>
            <td>
              <code>key:value</code>&nbsp;or&nbsp;<code>key:"value"</code>
            </td>
            <td width="100%">
              If no special chars(e.g. space, quotes, brackets) are required,
              format 1 can be used, otherwise use format 2, where
              <code>"</code> can be escaped by repeating it.
            </td>
          </tr>
          <tr>
            <th>Sub-queries</th>
            <td><code>@name:id:123</code></td>
            <td width="100%">
              Sub-queries are supported by prefixing any filter with
              <code>@subquery-name:</code>.
            </td>
          </tr>
          <tr>
            <th>Variables</th>
            <td><code>@id@</code> or <code>@subquery:ftime@</code></td>
            <td width="100%">
              Variables can be used within most filters. If subqueries are used,
              referencing a variable from a different subquery is done by
              prefixing the variablename with the subquery name and a
              <code>:</code>.
            </td>
          </tr>
          <tr>
            <th>Tag/Service/Mark/Generated&nbsp;filter</th>
            <td><code>tag:tagname,othertag</code>, <code>service:svc</code>, <code>mark:marked</code> or
              <code>generated:foo</code></td>
            <td width="100%">
              Restricts the results to streams that were identified as matching to
              the query of one of the named tags, services, generated or marks. Multiple names are separated by
              <code>,</code>.
            </td>
          </tr>
          <tr>
            <th>Protocol&nbsp;filter</th>
            <td><code>protocol:tcp,udp</code></td>
            <td width="100%">
              Restricts the results to streams of the given protocols, supported
              protocols are <code>tcp</code>, <code>udp</code> and
              <code>sctp</code>, separate the protocols by <code>,</code>. This
              filter supports the <code>protocol</code> variable, e.g.
              <code>protocol:@subquery:protocol@</code>.
            </td>
          </tr>
          <tr>
            <th>Id&nbsp;filter</th>
            <td><code>id:1,2,3,@subquery:id@+123</code></td>
            <td width="100%">
              Restricts the results to only streams with one of the given ids.
              You can give a list of (separate by <code>,</code>) ids or id
              ranges (using <code>:</code>), id ranges can be open(by leaving
              out the number) at any side. Any of these variables, optionally
              from subqueries, can be used: <code>id</code>,
              <code>[cs]port</code>, <code>[cs]bytes</code>. Simple calculations
              can be performed, using the operators <code>+</code> and
              <code>-</code>.
            </td>
          </tr>
          <tr>
            <th>Port&nbsp;filter</th>
            <td><code>[cs]port:80,1024:,</code></td>
            <td width="100%">
              <code>cport</code>, <code>sport</code> and
              <code>port</code> filter on the client, server or any port. The
              syntax is identical to the <code>id</code> filter syntax.
            </td>
          </tr>
          <tr>
            <th>Bytes&nbsp;filter</th>
            <td><code>[cs]bytes:1337,2048:4096</code></td>
            <td width="100%">
              <code>cbytes</code>, <code>sbytes</code> and
              <code>bytes</code> filter on the number of bytes send by the
              client, server or any of them. The syntax is identical to the
              <code>id</code> filter syntax.
            </td>
          </tr>
          <tr>
            <th>Host&nbsp;filter</th>
            <td>
              <code>[cs]host:1.2.3.4,10.0.0.0/8,::1,10.0.0.1/8/-8</code>
            </td>
            <td width="100%">
              <code>chost</code>, <code>shost</code> and
              <code>host</code> filter on the client, server or any host, lists
              are supported, each entry consists of an ip-address or a variable
              (e.g. <code>@subquery:[cs]host@</code>). Optionally, one or more
              <code>/bits</code> suffixes are appended. The suffixes can be
              negative, <code>/16/-8</code> would make a
              <code>255.255.0.255</code>/<code>ffff::ff</code> netmask.
            </td>
          </tr>
          <tr>
            <th>Time&nbsp;filter</th>
            <td>
              <code>[fl]time:-1h:,1300:1400,@subquery:ftime@-5m:</code>
            </td>
            <td width="100%">
              Filters to streams with the first(<code>ftime</code>),
              last(<code>ltime</code>) or any(<code>time</code>) packet being in
              the given timeranges. Lists are supported, you can use open ranges
              where each side of the range is either a relative time from now
              (e.g.
              <code>-2h3m4s</code>) or an absolute time using the format
              <code>[YYYY-MM-DD ]HHMM[SS]</code>.
              <code>[fl]time</code> variables can be used as well as simple
              calculations using <code>+</code> and <code>-</code>. For finding
              streams that lasted 5 minutes or longer you could e.g. use
              <code>ltime:@ftime@+5m</code>.
            </td>
          </tr>
          <tr>
            <th>Data&nbsp;filter</th>
            <td><code>[cs]data:flag[{}].+[}]</code></td>
            <td width="100%">
              Select streams that contain the given regex in the data send by
              the client(<code>cdata</code>), server(<code>sdata</code>) or any
              of them(<code>data</code>). The regex format is described here:
              <a href="https://golang.org/pkg/regexp/syntax/#hdr-Syntax" target="_blank">Golang regexp syntax</a>.
              Within one set of <code>then</code>-connected data filters, you
              can use variables referencing named capture groups from previous
              data filters of the same set. Example:
              <code>cdata:"(?P&lt;flag&gt;FLAG:[0-9a-f]{16})" then
                cdata:"@flag@"</code>. One set of <code>then</code>-connected
              <code>data</code> filters must belong to the same sub-query. A
              data filter can reference variables generated by sub queries that
              are <code>and</code> connected. E.g.
              <code>@sub:cdata:"the flag is (?P&lt;flag&gt;[0-9a-f]{16})"
                sdata:"@sub:flag@"</code>.
            </td>
          </tr>
          <tr>
            <th>Sorting</th>
            <td><code>sort:saddr,ftime,-id</code></td>
            <td width="100%">
              Results can be sorted by using the
              <code>sort</code> "filter". It may only appear once in the query,
              the value is a list of <code>,</code> separated terms with an
              optional <code>-</code> prefix inverting the sort order of that
              term. Available terms are: <code>id</code>, <code>[fl]time</code>,
              <code>[cs]bytes</code>, <code>[cs]host</code> and
              <code>[cs]port</code>. The default is <code>-ftime</code>.
            </td>
          </tr>
          <tr>
            <th>Limiting&nbsp;result&nbsp;count</th>
            <td><code>limit:10</code></td>
            <td width="100%">
              <code>limit</code> is used to restrict the number of results, it
              only accepts a number as value, the default is <code>100</code>,
              the value <code>0</code> means unlimited.
            </td>
          </tr>
          <tr>
            <th>Grouping</th>
            <td><code>group:"@sport@"</code></td>
            <td width="100%">
              Group the results by the variables listed in the arguments.
              Currently sub-query variables are not supported.
            </td>
          </tr>
        </tbody>
      </v-simple-table>
    </v-card-text>
  </v-card>
</template>

<style>

</style>

<script>
export default {
  name: "Home",
};
</script>